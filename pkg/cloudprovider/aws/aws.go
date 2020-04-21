package aws

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/atlassian/escalator/pkg/cloudprovider"
	"github.com/atlassian/escalator/pkg/metrics"
	awsapi "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

const (
	// ProviderName identifies this module as aws
	ProviderName = "aws"
	// LifecycleOnDemand string constant for On-Demand EC2 instances
	LifecycleOnDemand = "on-demand"
	// LifecycleSpot string constant for Spot EC2 instances
	LifecycleSpot = "spot"
	// The AttachInstances API only supports adding 20 instances at a time
	batchSize = 20
)

func instanceToProviderID(instance *autoscaling.Instance) string {
	return fmt.Sprintf("aws:///%s/%s", *instance.AvailabilityZone, *instance.InstanceId)
}

func providerIDToInstanceID(providerID string) string {
	return strings.Split(providerID, "/")[4]
}

// CloudProvider providers an aws cloud provider implementation
type CloudProvider struct {
	service    autoscalingiface.AutoScalingAPI
	ec2Service ec2iface.EC2API
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

// GetNodeGroup gets the node group from the cloud provider. Returns if it exists or not
func (c *CloudProvider) GetNodeGroup(id string) (cloudprovider.NodeGroup, bool) {
	ng, ok := c.nodeGroups[id]
	return ng, ok
}

// RegisterNodeGroups adds the nodegroup to the list of nodes groups
func (c *CloudProvider) RegisterNodeGroups(groups ...cloudprovider.NodeGroupConfig) error {
	configs := make(map[string]*cloudprovider.NodeGroupConfig, len(groups))
	strs := make([]*string, len(groups))
	for i, s := range groups {
		c := s
		strs[i] = awsapi.String(s.GroupID)
		configs[s.GroupID] = &c
	}

	input := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: strs,
	}

	result, err := c.service.DescribeAutoScalingGroups(input)
	if err != nil {
		log.Errorf("failed to describe asgs %v. err: %v", groups, err)
		return err
	}

	for _, group := range result.AutoScalingGroups {
		id := awsapi.StringValue(group.AutoScalingGroupName)
		if ng, ok := c.nodeGroups[id]; ok {
			// just update the group if it already exists
			ng.asg = group
			continue
		}

		c.nodeGroups[id] = NewNodeGroup(configs[id], group, c)
	}

	// Update metrics for each node group
	for _, nodeGroup := range c.nodeGroups {
		metrics.CloudProviderMinSize.WithLabelValues(c.Name(), nodeGroup.ID(), nodeGroup.Name()).Set(float64(nodeGroup.MinSize()))
		metrics.CloudProviderMaxSize.WithLabelValues(c.Name(), nodeGroup.ID(), nodeGroup.Name()).Set(float64(nodeGroup.MaxSize()))
		metrics.CloudProviderTargetSize.WithLabelValues(c.Name(), nodeGroup.ID(), nodeGroup.Name()).Set(float64(nodeGroup.TargetSize()))
		metrics.CloudProviderSize.WithLabelValues(c.Name(), nodeGroup.ID(), nodeGroup.Name()).Set(float64(nodeGroup.Size()))
	}

	return nil
}

// Refresh is called before every main loop and can be used to dynamically update cloud provider state.
func (c *CloudProvider) Refresh() error {
	configs := make([]cloudprovider.NodeGroupConfig, 0, len(c.nodeGroups))
	for _, ng := range c.nodeGroups {
		configs = append(configs, *ng.config)
	}

	return c.RegisterNodeGroups(configs...)
}

// Instance includes base EC2 instance information
type Instance struct {
	id          string
	ec2Instance *ec2.Instance
}

// GetInstance creates an Instance object through k8s Node object
func (c *CloudProvider) GetInstance(node *v1.Node) (cloudprovider.Instance, error) {
	var instance *Instance

	id := providerIDToInstanceID(node.Spec.ProviderID)

	input := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{&id},
	}

	result, err := c.ec2Service.DescribeInstances(input)

	if err != nil {
		log.Error("Error describing instance - ", err)
	} else {
		// There can be only one
		if len(result.Reservations) != 1 || len(result.Reservations[0].Instances) != 1 {
			err = errors.New("Malformed DescribeInstances response from AWS, expected only 1 Reservation and 1 Instance for id: " + id)
		} else {
			instance = &Instance{
				id:          id,
				ec2Instance: result.Reservations[0].Instances[0],
			}
		}
	}

	return instance, err
}

// InstantiationTime returns EC2 instance launch time
func (i *Instance) InstantiationTime() time.Time {
	return *i.ec2Instance.LaunchTime
}

// ID return EC2 instance ID
func (i *Instance) ID() string {
	return i.id
}

// NodeGroup implements a aws nodegroup
type NodeGroup struct {
	id   string
	name string
	asg  *autoscaling.Group

	provider *CloudProvider
	config   *cloudprovider.NodeGroupConfig
}

// NewNodeGroup creates a new nodegroup from the aws group backing
func NewNodeGroup(config *cloudprovider.NodeGroupConfig, asg *autoscaling.Group, provider *CloudProvider) *NodeGroup {
	return &NodeGroup{
		id:       config.GroupID,
		name:     config.Name,
		asg:      asg,
		provider: provider,
		config:   config,
	}
}

func (n *NodeGroup) String() string {
	return fmt.Sprint(n.asg)
}

// ID returns an unique identifier of the node group.
func (n *NodeGroup) ID() string {
	return n.id
}

// Name returns the name of the node group for this cloud provider node group.
func (n *NodeGroup) Name() string {
	return n.name
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

// canScaleInOneShot return value indicates if the cloud provider is configured
// to support one-shot scaling
func (n *NodeGroup) canScaleInOneShot() bool {
	return n.config.AWSConfig.LaunchTemplateID != ""
}

// IncreaseSize increases the size of the node group. To delete a node you need
// to explicitly name it and use DeleteNode. This function should wait until
// node group size is updated.
func (n *NodeGroup) IncreaseSize(delta int64) error {
	if delta <= 0 {
		return fmt.Errorf("size increase must be positive")
	}

	if n.TargetSize()+delta > n.MaxSize() {
		return fmt.Errorf("increasing size will breach maximum node size")
	}

	log.WithField("asg", n.id).Debugf("IncreaseSize: %v", delta)

	if n.canScaleInOneShot() {
		log.WithField("asg", n.id).Infof("Scaling with CreateFleet strategy")
		return n.setASGDesiredSizeOneShot(delta)
	}

	log.WithField("asg", n.id).Infof("Scaling with SetDesiredCapacity strategy")
	return n.setASGDesiredSize(n.TargetSize() + delta)

}

// DeleteNodes deletes nodes from this node group. Error is returned either on
// failure or if the given node doesn't belong to this node group. This function
// should wait until node group size is updated.
func (n *NodeGroup) DeleteNodes(nodes ...*v1.Node) error {
	if n.TargetSize() <= n.MinSize() {
		return fmt.Errorf("min sized reached, nodes will not be deleted")
	}

	if n.TargetSize()-int64(len(nodes)) < n.MinSize() {
		return fmt.Errorf("terminating nodes will breach minimum node size")
	}

	for _, node := range nodes {
		if !n.Belongs(node) {
			log.Debugf("instances in ASG: %v", n.Nodes())
			return &cloudprovider.NodeNotInNodeGroup{NodeName: node.Name, ProviderID: node.Spec.ProviderID, NodeGroup: n.ID()}
		}

		// find which instance this is
		var instanceID *string
		for _, instance := range n.asg.Instances {
			if node.Spec.ProviderID == instanceToProviderID(instance) {
				instanceID = instance.InstanceId
				break
			}
		}

		input := &autoscaling.TerminateInstanceInAutoScalingGroupInput{
			InstanceId:                     instanceID,
			ShouldDecrementDesiredCapacity: awsapi.Bool(true),
		}

		result, err := n.provider.service.TerminateInstanceInAutoScalingGroup(input)
		if err != nil {
			return fmt.Errorf("failed to terminate instance. err: %v", err)
		}
		log.Debug(*result.Activity.Description)
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

	if n.TargetSize()+delta < n.MinSize() {
		return fmt.Errorf("decreasing target size will breach minimum node size")
	}

	log.WithField("asg", n.id).Debugf("DecreaseTargetSize: %v", delta)
	return n.setASGDesiredSize(n.TargetSize() + delta)
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
// user must make sure that newSize is not out of bounds of the asg
func (n *NodeGroup) setASGDesiredSize(newSize int64) error {
	input := &autoscaling.SetDesiredCapacityInput{
		AutoScalingGroupName: awsapi.String(n.id),
		DesiredCapacity:      awsapi.Int64(newSize),
		HonorCooldown:        awsapi.Bool(false),
	}

	log.WithField("asg", n.id).Debugf("SetDesiredCapacity: %v", newSize)
	log.WithField("asg", n.id).Debugf("CurrentSize: %v", n.Size())
	log.WithField("asg", n.id).Debugf("CurrentTargetSize: %v", n.TargetSize())
	_, err := n.provider.service.SetDesiredCapacity(input)
	return err
}

// setASGDesiredSizeOneShot uses the AWS fleet API to acquire all desired
// capacity in one step and then add it to the existing auto-scaling group.
func (n *NodeGroup) setASGDesiredSizeOneShot(addCount int64) error {
	// Parse the Escalator args into the correct format for a CreateFleet request, then make the request.
	fleetInput, err := createFleetInput(*n, addCount)
	if err != nil {
		log.Error("Failed setup for CreateFleet call.")
		return err
	}

	fleet, err := n.provider.ec2Service.CreateFleet(fleetInput)
	if err != nil {
		log.Errorf("Failed CreateFleet call. CreateFleetInput: %v", fleetInput)
		return err
	}

	// CreateFleet returns an array of errors with the response. Sometimes errors are present even when instances were
	// successfully provisioned. In this case, the min target capacity is the size of the full request, so if any
	// instances are present this indicates we got them all and can ignore the errors.
	if len(fleet.Instances) == 0 && len(fleet.Errors) > 0 {
		for _, err := range fleet.Errors {
			log.Error(*err.ErrorMessage)
		}
		return errors.New(*fleet.Errors[0].ErrorMessage)
	}

	instances := make([]*string, 0)
	for _, i := range fleet.Instances {
		instances = append(instances, i.InstanceIds...)
	}

	ticker := time.NewTicker(1 * time.Second)
	deadline := time.NewTimer(n.config.AWSConfig.FleetInstanceReadyTimeout)
	defer ticker.Stop()
	defer deadline.Stop()

	// Escalator will block waiting for all nodes to become available in this
	// node group for the maximum time specified in FleetInstanceReadyTimeout.
	// This should typically be quite fast as it's just the time for the
	// instance to boot and transition to ready state. The instance must be in
	// ready state before AttachInstances will graft it onto an ASG.
InstanceReadyLoop:
	for {
		select {
		case <-ticker.C:
			if n.allInstancesReady(instances) {
				break InstanceReadyLoop
			}
		case <-deadline.C:
			return errors.New("Not all instances could be started")
		}
	}

	var batch []*string
	for batchSize < len(instances) {
		instances, batch = instances[batchSize:], instances[0:batchSize:batchSize]

		_, err = n.provider.service.AttachInstances(&autoscaling.AttachInstancesInput{
			AutoScalingGroupName: awsapi.String(n.id),
			InstanceIds:          batch,
		})
		if err != nil {
			log.Error("Failed AttachInstances call.")
			return err
		}
	}

	// Attach the remainder for instance sets that are not evenly divisible by
	// batchSize
	_, err = n.provider.service.AttachInstances(&autoscaling.AttachInstancesInput{
		AutoScalingGroupName: awsapi.String(n.id),
		InstanceIds:          instances,
	})

	log.WithField("asg", n.id).Debugf("CurrentSize: %v", n.Size())
	log.WithField("asg", n.id).Debugf("CurrentTargetSize: %v", n.TargetSize())
	if err != nil {
		log.Error("Failed AttachInstances call.")
		return err
	}

	return nil
}

func (n *NodeGroup) allInstancesReady(ids []*string) bool {
	ready := false

	n.provider.ec2Service.DescribeInstanceStatusPages(&ec2.DescribeInstanceStatusInput{
		InstanceIds:         ids,
		IncludeAllInstances: awsapi.Bool(true),
	}, func(r *ec2.DescribeInstanceStatusOutput, lastPage bool) bool {
		for _, i := range r.InstanceStatuses {
			if *i.InstanceState.Name != "running" {
				return false
			}
		}

		// If we made it to the last page and didn't bail early then all
		// instances are ready
		if lastPage {
			ready = true
		}

		return true
	})

	return ready
}

// createFleetInput will parse Escalator input into the format needed for a CreateFleet request.
func createFleetInput(n NodeGroup, addCount int64) (*ec2.CreateFleetInput, error) {
	lifecycle := n.config.AWSConfig.Lifecycle
	if lifecycle == "" {
		lifecycle = LifecycleOnDemand
	}

	launchTemplateOverrides, err := createTemplateOverrides(n)
	if err != nil {
		return nil, err
	}

	fleetInput := &ec2.CreateFleetInput{
		Type:                             awsapi.String("instant"),
		TerminateInstancesWithExpiration: awsapi.Bool(false),
		TargetCapacitySpecification: &ec2.TargetCapacitySpecificationRequest{
			TotalTargetCapacity:       awsapi.Int64(addCount),
			DefaultTargetCapacityType: awsapi.String(lifecycle),
		},
		LaunchTemplateConfigs: []*ec2.FleetLaunchTemplateConfigRequest{
			{
				LaunchTemplateSpecification: &ec2.FleetLaunchTemplateSpecificationRequest{
					LaunchTemplateId: awsapi.String(n.config.AWSConfig.LaunchTemplateID),
					Version:          awsapi.String(n.config.AWSConfig.LaunchTemplateVersion),
				},
				Overrides: launchTemplateOverrides,
			},
		},
	}

	if lifecycle == LifecycleOnDemand {
		fleetInput.OnDemandOptions = &ec2.OnDemandOptionsRequest{
			MinTargetCapacity:  awsapi.Int64(addCount),
			SingleInstanceType: awsapi.Bool(true),
		}
	} else {
		fleetInput.SpotOptions = &ec2.SpotOptionsRequest{
			MinTargetCapacity:  awsapi.Int64(addCount),
			SingleInstanceType: awsapi.Bool(true),
		}
	}

	return fleetInput, nil
}

// createTemplateOverrides will parse the overrides into the FleetLaunchTemplateOverridesRequest format
func createTemplateOverrides(n NodeGroup) ([]*ec2.FleetLaunchTemplateOverridesRequest, error) {
	// Get subnetIDs from the ASG
	describeASGOutput, err := n.provider.service.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{
			awsapi.String(n.id),
		},
	})
	if err != nil {
		log.Errorf("Failed call to DescribeAutoScalingGroups for ASG %v", n.id)
		return nil, err
	}
	if len(describeASGOutput.AutoScalingGroups) == 0 {
		return nil, errors.New("failed to get an ASG from DescribeAutoscalingGroups response")
	}
	if *describeASGOutput.AutoScalingGroups[0].VPCZoneIdentifier == "" {
		return nil, errors.New("failed to get any subnetIDs from DescribeAutoscalingGroups response")
	}
	vpcZoneIdentifier := describeASGOutput.AutoScalingGroups[0].VPCZoneIdentifier
	subnetIDs := strings.Split(*vpcZoneIdentifier, ",")

	instanceTypes := n.config.AWSConfig.InstanceTypeOverrides

	var launchTemplateOverrides []*ec2.FleetLaunchTemplateOverridesRequest
	if len(instanceTypes) > 0 {
		for i := range subnetIDs {
			for j := range instanceTypes {
				overridesRequest := ec2.FleetLaunchTemplateOverridesRequest{
					SubnetId:     &subnetIDs[i],
					InstanceType: &instanceTypes[j],
				}
				launchTemplateOverrides = append(launchTemplateOverrides, &overridesRequest)
			}
		}
	} else {
		for i := range subnetIDs {
			overridesRequest := ec2.FleetLaunchTemplateOverridesRequest{
				SubnetId: &subnetIDs[i],
			}
			launchTemplateOverrides = append(launchTemplateOverrides, &overridesRequest)
		}
	}

	return launchTemplateOverrides, nil
}
