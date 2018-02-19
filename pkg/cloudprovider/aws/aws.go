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

// ErrorNotImplemented indicates that a method or function has not been implemented yet
var ErrorNotImplemented = fmt.Errorf("method not implemented")

// CloudProvider providers an aws cloudprovider implementation
type CloudProvider struct {
	service    *autoscaling.AutoScaling
	nodeGroups []*NodeGroup
}

// Name returns name of the cloud provider.
func (c *CloudProvider) Name() string {
	return ProviderName
}

// NodeGroups returns all node groups configured for this cloud provider.
func (c *CloudProvider) NodeGroups() []cloudprovider.NodeGroup {
	// put the nodegroup concrete type into the abstract type
	ngs := make([]cloudprovider.NodeGroup, len(c.nodeGroups))
	for i, ng := range c.nodeGroups {
		ngs[i] = ng
	}
	return ngs
}

// GetNodeGroup gets the node group from the coudprovider. Returns if it exists or not
func (c *CloudProvider) GetNodeGroup(id string) (cloudprovider.NodeGroup, bool) {
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

	for i, group := range result.AutoScalingGroups {
		c.nodeGroups = append(c.nodeGroups, NewNodeGroup(
			ids[i],
			group,
		))
	}

	return nil
}

// Refresh is called before every main loop and can be used to dynamically update cloud provider state.
func (c *CloudProvider) Refresh() error {
	return ErrorNotImplemented
}

// Cleanup cleans up open resources before the cloud provider is destroyed, i.e. go routines etc.
func (c *CloudProvider) Cleanup() error {
	return ErrorNotImplemented
}

// NodeGroup implements a aws nodegroup
type NodeGroup struct {
	id  string
	asg *autoscaling.Group
}

// NewNodeGroup creates a new nodegroup from the aws group backing
func NewNodeGroup(id string, asg *autoscaling.Group) *NodeGroup {
	return &NodeGroup{
		id:  id,
		asg: asg,
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
	if n.asg.MinSize == nil {
		return 0
	}
	return *n.asg.MinSize
}

// MaxSize returns maximum size of the node group.
func (n *NodeGroup) MaxSize() int64 {
	if n.asg.MaxSize == nil {
		return 0
	}
	return *n.asg.MaxSize
}

// TargetSize returns the current target size of the node group. It is possible that the
// number of nodes in Kubernetes is different at the moment but should be equal
// to Size() once everything stabilizes (new nodes finish startup and registration or
// removed nodes are deleted completely).
func (n *NodeGroup) TargetSize() int64 {
	if n.asg.DesiredCapacity == nil {
		// TODO(jgonzalez): handle error condition better
		return 0
	}
	return *n.asg.DesiredCapacity
}

// IncreaseSize increases the size of the node group. To delete a node you need
// to explicitly name it and use DeleteNode. This function should wait until
// node group size is updated.
func (n *NodeGroup) IncreaseSize(delta int64) error {
	return ErrorNotImplemented
}

// DeleteNodes deletes nodes from this node group. Error is returned either on
// failure or if the given node doesn't belong to this node group. This function
// should wait until node group size is updated.
func (n *NodeGroup) DeleteNodes(...*v1.Node) error {
	return ErrorNotImplemented
}

// DecreaseTargetSize decreases the target size of the node group. This function
// doesn't permit to delete any existing node and can be used only to reduce the
// request for new nodes that have not been yet fulfilled. Delta should be negative.
// It is assumed that cloud provider will not delete the existing nodes when there
// is an option to just decrease the target.
func (n *NodeGroup) DecreaseTargetSize(delta int) error {
	return ErrorNotImplemented
}

// Nodes returns a list of all nodes that belong to this node group.
func (n *NodeGroup) Nodes() ([]string, error) {
	return nil, ErrorNotImplemented
}
