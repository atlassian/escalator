package aws

import (
	"fmt"

	"github.com/atlassian/escalator/pkg/cloudprovider"
	awsapi "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
)

// ProviderName identifies this module as aws
const ProviderName = "aws"

func instanceToProviderID(instance *autoscaling.Instance) string {
	return fmt.Sprintf("aws:///%s/%s", *instance.AvailabilityZone, *instance.InstanceId)
}

// CloudProvider providers an aws cloudprovider implementation
type CloudProvider struct {
	service    *autoscaling.AutoScaling
	nodeGroups map[string]*NodeGroup
}

// Name returns name of the cloud provider.
func (c *CloudProvider) Name() string {
	return ProviderName
}

// NodeGroups returns all node groups configured for this cloud provider.
func (c *CloudProvider) NodeGroups() []cloudprovider.NodeGroup {
	// put the nodegroup concrete type into the abstract type
	ngs := make([]cloudprovider.NodeGroup, 0, len(c.nodeGroups))
	for _, ng := range c.nodeGroups {
		ngs = append(ngs, ng)
	}
	return ngs
}

// GetNodeGroup gets the node group from the coudprovider. Returns if it exists or not
func (c *CloudProvider) GetNodeGroup(id string) (cloudprovider.NodeGroup, bool) {
	if ng, ok := c.nodeGroups[id]; ok {
		return ng, ok
	}
	return nil, false
}

// RegisterNodeGroups adds the nodegroup to the list of nodes groups
func (c *CloudProvider) RegisterNodeGroups(ids ...string) error {
	strs := make([]*string, len(ids))
	for i, s := range ids {
		strs[i] = awsapi.String(s)
	}

	input := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: strs,
	}

	result, err := c.service.DescribeAutoScalingGroups(input)
	if err != nil {
		log.Errorf("failed to describe asgs %v. err: %v", ids, err)
		return err
	}

	for _, group := range result.AutoScalingGroups {
		id := awsapi.StringValue(group.AutoScalingGroupName)
		if ng, ok := c.nodeGroups[id]; ok {
			// just update the group if it already exists
			ng.asg = group
			continue
		}

		c.nodeGroups[id] = NewNodeGroup(
			id,
			group,
			c,
		)
	}

	return nil
}

// Refresh is called before every main loop and can be used to dynamically update cloud provider state.
func (c *CloudProvider) Refresh() error {
	ids := make([]string, 0, len(c.nodeGroups))
	for id := range c.nodeGroups {
		ids = append(ids, id)
	}

	return c.RegisterNodeGroups(ids...)
}

// NodeGroup implements a aws nodegroup
type NodeGroup struct {
	id  string
	asg *autoscaling.Group

	provider *CloudProvider
}

// NewNodeGroup creates a new nodegroup from the aws group backing
func NewNodeGroup(id string, asg *autoscaling.Group, provider *CloudProvider) *NodeGroup {
	return &NodeGroup{
		id:       id,
		asg:      asg,
		provider: provider,
	}
}

func (n *NodeGroup) String() string {
	return fmt.Sprint(n.asg)
}

// ID returns an unique identifier of the node group.
func (n *NodeGroup) ID() string {
	return n.id
}

// MinSize returns minimum size of the node group.
func (n *NodeGroup) MinSize() int64 {
	return awsapi.Int64Value(n.asg.MinSize)
}

// MaxSize returns maximum size of the node group.
func (n *NodeGroup) MaxSize() int64 {
	return awsapi.Int64Value(n.asg.MaxSize)
}

// TargetSize returns the current target size of the node group. It is possible that the
// number of nodes in Kubernetes is different at the moment but should be equal
// to Size() once everything stabilizes (new nodes finish startup and registration or
// removed nodes are deleted completely).
func (n *NodeGroup) TargetSize() int64 {
	return awsapi.Int64Value(n.asg.DesiredCapacity)
}

// Size is the number of instances in the nodegroup at the current time
func (n *NodeGroup) Size() int64 {
	return int64(len(n.asg.Instances))
}

// IncreaseSize increases the size of the node group. To delete a node you need
// to explicitly name it and use DeleteNode. This function should wait until
// node group size is updated.
func (n *NodeGroup) IncreaseSize(delta int64) error {
	if delta <= 0 {
		return fmt.Errorf("size increase must be positive")
	}

	size := n.Size()

	// This means that a scaling action is likely to be in progress and aws will reject the new api call until it's done
	if size != n.TargetSize() {
		return fmt.Errorf("Must wait until size(%v) == target(%v)", size, n.TargetSize())
	}

	return n.setASGDesiredSize(size + delta)
}

// DeleteNodes deletes nodes from this node group. Error is returned either on
// failure or if the given node doesn't belong to this node group. This function
// should wait until node group size is updated.
func (n *NodeGroup) DeleteNodes(nodes ...*v1.Node) error {
	if n.Size() <= n.MinSize() {
		return fmt.Errorf("min sized reached, nodes will not be deleted")
	}

	for _, node := range nodes {
		if !n.Belongs(node) {
			return fmt.Errorf("node %v belongs in a different asg than %v", node.Name, n.ID())
		}

		// find which instance this is
		var instanceID *string
		for _, instance := range n.asg.Instances {
			if node.Spec.ProviderID == instanceToProviderID(instance) {
				instanceID = instance.InstanceId
			}
		}

		if instanceID == nil {
			return fmt.Errorf("failed to match node id (%v) to an aws instance id", node.Spec.ProviderID)
		}

		input := &autoscaling.TerminateInstanceInAutoScalingGroupInput{
			InstanceId:                     instanceID,
			ShouldDecrementDesiredCapacity: awsapi.Bool(true),
		}

		result, err := n.provider.service.TerminateInstanceInAutoScalingGroup(input)
		if err != nil {
			return fmt.Errorf("failed to terminate instance. err: %v", err)
		}
		log.Debugln(result.Activity.Description)
	}

	return nil
}

// Belongs determines if the node belongs in the current node group
func (n *NodeGroup) Belongs(node *v1.Node) bool {
	nodeProviderID := node.Spec.ProviderID

	for _, id := range n.Nodes() {
		if id == nodeProviderID {
			return true
		}
	}

	return false
}

// DecreaseTargetSize decreases the target size of the node group. This function
// doesn't permit to delete any existing node and can be used only to reduce the
// request for new nodes that have not been yet fulfilled. Delta should be negative.
// It is assumed that cloud provider will not delete the existing nodes when there
// is an option to just decrease the target.
func (n *NodeGroup) DecreaseTargetSize(delta int64) error {
	if delta >= 0 {
		return fmt.Errorf("size decrease delta must be negative")
	}
	size := n.Size()
	nodes := n.Nodes()

	if size+delta < int64(len(nodes)) {
		return fmt.Errorf("attempt to delete existing nodes targetSize:%v delta:%v existingNodes: %v",
			size, delta, len(nodes))
	}
	return n.setASGDesiredSize(size + delta)
}

// Nodes returns a list of all nodes that belong to this node group.
func (n *NodeGroup) Nodes() []string {
	result := make([]string, 0, len(n.asg.Instances))
	for _, instance := range n.asg.Instances {
		result = append(result, instanceToProviderID(instance))
	}

	return result
}

// setASGDesiredSize sets the asg desired size to the new size
func (n *NodeGroup) setASGDesiredSize(newSize int64) error {
	if newSize < n.MinSize() {
		return fmt.Errorf("attempt to set desired capacity (%v) below minimum (%v)", newSize, n.MinSize())
	}
	if newSize > n.MaxSize() {
		return fmt.Errorf("attempt to set desired capacity (%v) above maximum (%v)", newSize, n.MaxSize())
	}

	input := &autoscaling.SetDesiredCapacityInput{
		AutoScalingGroupName: awsapi.String(n.id),
		DesiredCapacity:      awsapi.Int64(newSize),
		HonorCooldown:        awsapi.Bool(true),
	}

	result, err := n.provider.service.SetDesiredCapacity(input)
	if err != nil {
		if err != nil {
			log.Errorf("failed to set asg size: %v", err)
			return err
		}
	}

	log.Debugln("result returned:", result)

	return nil
}
