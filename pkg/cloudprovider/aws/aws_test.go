package aws

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"

	"github.com/atlassian/escalator/pkg/cloudprovider"
	"github.com/atlassian/escalator/pkg/test"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stretchr/testify/assert"
)

// Arbitrarily longer than the Ticker timeout in setASGDesiredSizeOneShot
const tickerTimeout = 2 * time.Second

var (
	mockASG             = autoscaling.Group{}
	mockAWSConfig       = cloudprovider.AWSNodeGroupConfig{}
	mockNodeGroup       = NodeGroup{}
	mockNodeGroupConfig = cloudprovider.NodeGroupConfig{}
)

func setupAWSMocks() {
	mockASG = autoscaling.Group{
		AutoScalingGroupName: aws.String("asg-1"),
		MaxSize:              aws.Int64(int64(25)),
		DesiredCapacity:      aws.Int64(int64(1)),
		VPCZoneIdentifier:    aws.String("subnetID-1,subnetID-2"),
		Tags:                 []*autoscaling.TagDescription{},
	}

	mockAWSConfig = cloudprovider.AWSNodeGroupConfig{
		LaunchTemplateID:          "lt-123456",
		LaunchTemplateVersion:     "1",
		FleetInstanceReadyTimeout: tickerTimeout,
		Lifecycle:                 LifecycleOnDemand,
		InstanceTypeOverrides:     []string{"instance-1", "instance-2"},
		ResourceTagging:           false,
	}

	mockNodeGroup = NodeGroup{
		id:     "id",
		name:   "name",
		config: &mockNodeGroupConfig,
		asg:    &mockASG,
	}

	mockNodeGroupConfig = cloudprovider.NodeGroupConfig{
		Name:      "nodeGroupConfig",
		GroupID:   "",
		AWSConfig: mockAWSConfig,
	}
}

func TestInstanceToProviderId(t *testing.T) {
	instance := &autoscaling.Instance{
		AvailabilityZone: aws.String("us-east-1b"),
		InstanceId:       aws.String("abc123"),
	}
	res := instanceToProviderID(instance)
	assert.Equal(t, "aws:///us-east-1b/abc123", res)
}

func TestProviderIdToInstanceId(t *testing.T) {
	assert.Equal(t, "abc123", providerIDToInstanceID("aws:///us-east-1b/abc123"))
}

func newMockCloudProvider(nodeGroups []string, service *test.MockAutoscalingService, ec2Service *test.MockEc2Service) (*CloudProvider, error) {
	var err error

	cloudProvider := &CloudProvider{
		service:    service,
		ec2Service: ec2Service,
		nodeGroups: make(map[string]*NodeGroup, len(nodeGroups)),
	}

	configs := make([]cloudprovider.NodeGroupConfig, 0, len(nodeGroups))
	for _, i := range nodeGroups {
		configs = append(configs, cloudprovider.NodeGroupConfig{GroupID: i})
	}

	if service != nil {
		err = cloudProvider.RegisterNodeGroups(configs...)
	}

	return cloudProvider, err
}

// Similar to newMockCloudProvider but node groups are injected instead of created within this function
func newMockCloudProviderUsingInjection(nodeGroups map[string]*NodeGroup, service autoscalingiface.AutoScalingAPI, ec2Service ec2iface.EC2API) (*CloudProvider, error) {
	var err error

	cloudProvider := &CloudProvider{
		service:    service,
		ec2Service: ec2Service,
		nodeGroups: nodeGroups,
	}

	for _, nodeGroup := range nodeGroups {
		nodeGroup.provider = cloudProvider
	}

	return cloudProvider, err
}

func TestCreateFleetInput(t *testing.T) {
	setupAWSMocks()
	lifecycles := []string{"", LifecycleOnDemand, LifecycleSpot}
	for _, lifecycle := range lifecycles {
		autoScalingGroups := []*autoscaling.Group{&mockASG}
		nodeGroups := map[string]*NodeGroup{mockNodeGroup.id: &mockNodeGroup}
		addCount := int64(2)
		mockAWSConfig.Lifecycle = lifecycle

		awsCloudProvider, _ := newMockCloudProviderUsingInjection(
			nodeGroups,
			&test.MockAutoscalingService{
				DescribeAutoScalingGroupsOutput: &autoscaling.DescribeAutoScalingGroupsOutput{
					AutoScalingGroups: autoScalingGroups,
				},
			},
			&test.MockEc2Service{},
		)
		mockNodeGroup.provider = awsCloudProvider
		mockNodeGroupConfig.AWSConfig = mockAWSConfig

		_, err := createFleetInput(mockNodeGroup, addCount)
		assert.Nil(t, err, "Expected no error from createFleetInput")
	}
}

func TestCreateFleetInput_WithResourceTagging(t *testing.T) {
	setupAWSMocks()
	autoScalingGroups := []*autoscaling.Group{&mockASG}
	nodeGroups := map[string]*NodeGroup{mockNodeGroup.id: &mockNodeGroup}
	addCount := int64(2)

	awsCloudProvider, _ := newMockCloudProviderUsingInjection(
		nodeGroups,
		&test.MockAutoscalingService{
			DescribeAutoScalingGroupsOutput: &autoscaling.DescribeAutoScalingGroupsOutput{
				AutoScalingGroups: autoScalingGroups,
			},
		},
		&test.MockEc2Service{},
	)
	mockNodeGroup.provider = awsCloudProvider
	mockAWSConfig.ResourceTagging = true
	mockNodeGroupConfig.AWSConfig = mockAWSConfig

	_, err := createFleetInput(mockNodeGroup, addCount)
	assert.Nil(t, err, "Expected no error from createFleetInput")
}

func TestCreateTemplateOverrides_FailedCall(t *testing.T) {
	setupAWSMocks()
	expectedError := errors.New("call failed")
	nodeGroups := map[string]*NodeGroup{mockNodeGroup.id: &mockNodeGroup}

	awsCloudProvider, _ := newMockCloudProviderUsingInjection(
		nodeGroups,
		&test.MockAutoscalingService{
			DescribeAutoScalingGroupsOutput: &autoscaling.DescribeAutoScalingGroupsOutput{},
			DescribeAutoScalingGroupsErr:    expectedError,
		},
		&test.MockEc2Service{},
	)
	mockNodeGroup.provider = awsCloudProvider

	_, err := createTemplateOverrides(mockNodeGroup)
	assert.Equal(t, expectedError, err, "Expected error with message '%v'", expectedError)
}

func TestCreateTemplateOverrides_NoASG(t *testing.T) {
	setupAWSMocks()
	var autoScalingGroups []*autoscaling.Group
	nodeGroups := map[string]*NodeGroup{mockNodeGroup.id: &mockNodeGroup}

	awsCloudProvider, _ := newMockCloudProviderUsingInjection(
		nodeGroups,
		&test.MockAutoscalingService{
			DescribeAutoScalingGroupsOutput: &autoscaling.DescribeAutoScalingGroupsOutput{
				AutoScalingGroups: autoScalingGroups,
			},
		},
		&test.MockEc2Service{},
	)
	mockNodeGroup.provider = awsCloudProvider

	_, error := createTemplateOverrides(mockNodeGroup)
	errorMessage := "failed to get an ASG from DescribeAutoscalingGroups response"
	e := errors.New(errorMessage)
	assert.Equalf(t, e, error, "Expected error with message '%v'", errorMessage)
}

func TestCreateTemplateOverrides_NoSubnetIDs(t *testing.T) {
	setupAWSMocks()
	subnetIDs := ""
	mockASG.VPCZoneIdentifier = &subnetIDs
	autoScalingGroups := []*autoscaling.Group{&mockASG}
	nodeGroups := map[string]*NodeGroup{mockNodeGroup.id: &mockNodeGroup}

	awsCloudProvider, _ := newMockCloudProviderUsingInjection(
		nodeGroups,
		&test.MockAutoscalingService{
			DescribeAutoScalingGroupsOutput: &autoscaling.DescribeAutoScalingGroupsOutput{
				AutoScalingGroups: autoScalingGroups,
			},
		},
		&test.MockEc2Service{},
	)
	mockNodeGroup.provider = awsCloudProvider

	_, error := createTemplateOverrides(mockNodeGroup)
	errorMessage := "failed to get any subnetIDs from DescribeAutoscalingGroups response"
	e := errors.New(errorMessage)
	assert.Equalf(t, e, error, "Expected error with message '%v'", errorMessage)
}

func TestCreateTemplateOverrides_Success(t *testing.T) {
	setupAWSMocks()
	autoScalingGroups := []*autoscaling.Group{&mockASG}
	nodeGroups := map[string]*NodeGroup{mockNodeGroup.id: &mockNodeGroup}

	awsCloudProvider, _ := newMockCloudProviderUsingInjection(
		nodeGroups,
		&test.MockAutoscalingService{
			DescribeAutoScalingGroupsOutput: &autoscaling.DescribeAutoScalingGroupsOutput{
				AutoScalingGroups: autoScalingGroups,
			},
		},
		&test.MockEc2Service{},
	)
	mockNodeGroup.provider = awsCloudProvider

	_, err := createTemplateOverrides(mockNodeGroup)
	assert.Nil(t, err, "Expected no error from createTemplateOverrides")
}

func TestCreateTemplateOverrides_NoInstanceTypeOverrides_Success(t *testing.T) {
	setupAWSMocks()
	autoScalingGroups := []*autoscaling.Group{&mockASG}
	nodeGroups := map[string]*NodeGroup{mockNodeGroup.id: &mockNodeGroup}

	awsCloudProvider, _ := newMockCloudProviderUsingInjection(
		nodeGroups,
		&test.MockAutoscalingService{
			DescribeAutoScalingGroupsOutput: &autoscaling.DescribeAutoScalingGroupsOutput{
				AutoScalingGroups: autoScalingGroups,
			},
		},
		&test.MockEc2Service{},
	)
	mockNodeGroup.provider = awsCloudProvider
	mockAWSConfig.InstanceTypeOverrides = nil
	mockNodeGroupConfig.AWSConfig = mockAWSConfig

	_, err := createTemplateOverrides(mockNodeGroup)
	assert.Nil(t, err, "Expected no error from createTemplateOverrides")
}
func TestAddASGTags_ResourceTaggingFalse(t *testing.T) {
	setupAWSMocks()
	mockNodeGroupConfig.AWSConfig.ResourceTagging = false
	awsCloudProvider, _ := newMockCloudProviderUsingInjection(
		nil,
		&test.MockAutoscalingService{},
		&test.MockEc2Service{},
	)
	addASGTags(&mockNodeGroupConfig, &mockASG, awsCloudProvider)
}

func TestAddASGTags_ResourceTaggingTrue(t *testing.T) {
	setupAWSMocks()
	mockNodeGroupConfig.AWSConfig.ResourceTagging = true

	// Mock service call
	awsCloudProvider, _ := newMockCloudProviderUsingInjection(
		nil,
		&test.MockAutoscalingService{
			CreateOrUpdateTagsOutput: &autoscaling.CreateOrUpdateTagsOutput{},
		},
		&test.MockEc2Service{},
	)
	addASGTags(&mockNodeGroupConfig, &mockASG, awsCloudProvider)
}

func TestAddASGTags_ASGAlreadyTagged(t *testing.T) {
	setupAWSMocks()
	mockNodeGroupConfig.AWSConfig.ResourceTagging = true

	// Mock existing tags
	key := tagKey
	asgTag := autoscaling.TagDescription{
		Key: &key,
	}
	mockASG.Tags = append(mockASG.Tags, &asgTag)

	// Mock service call
	awsCloudProvider, _ := newMockCloudProviderUsingInjection(
		nil,
		&test.MockAutoscalingService{
			CreateOrUpdateTagsOutput: &autoscaling.CreateOrUpdateTagsOutput{},
		},
		&test.MockEc2Service{},
	)
	addASGTags(&mockNodeGroupConfig, &mockASG, awsCloudProvider)
}

func TestAddASGTags_WithErrorResponse(t *testing.T) {
	setupAWSMocks()
	mockNodeGroupConfig.AWSConfig.ResourceTagging = true

	// Mock service call and error
	awsCloudProvider, _ := newMockCloudProviderUsingInjection(
		nil,
		&test.MockAutoscalingService{
			CreateOrUpdateTagsOutput: &autoscaling.CreateOrUpdateTagsOutput{},
			CreateOrUpdateTagsErr:    errors.New("unauthorized"),
		},
		&test.MockEc2Service{},
	)
	addASGTags(&mockNodeGroupConfig, &mockASG, awsCloudProvider)
}

// local mock of ASG service with custom AttachInstances method
type mockAutoscalingService struct {
	autoscalingiface.AutoScalingAPI
	*client.Client

	numCalls    int
	numMaxCalls int
}

// fail the AttachInstances call after numMaxCalls have happened
func (m *mockAutoscalingService) AttachInstances(*autoscaling.AttachInstancesInput) (*autoscaling.AttachInstancesOutput, error) {
	if m.numCalls < m.numMaxCalls {
		m.numCalls++
		return &autoscaling.AttachInstancesOutput{}, nil
	}
	return &autoscaling.AttachInstancesOutput{}, fmt.Errorf("failed the AttachInstances call")
}

func TestAttachInstancesToASG_WithInstanceReadyTimeout_ExpectFailure(t *testing.T) {
	setupAWSMocks()
	instanceID := "instanceID"

	numInstances := 123
	var instanceIDs []*string
	for i := 0; i < numInstances; i++ {
		instanceIDs = append(instanceIDs, &instanceID)
	}

	// Mock service call and error
	awsCloudProvider, _ := newMockCloudProviderUsingInjection(
		nil,
		&mockAutoscalingService{
			AutoScalingAPI: nil,
			Client:         nil,
			numCalls:       0,
			numMaxCalls:    2,
		},
		&test.MockEc2Service{
			TerminateInstancesOutput: &ec2.TerminateInstancesOutput{},
			TerminateInstancesErr:    nil,
			AllInstancesReady:        false,
		},
	)

	mockNodeGroup.provider = awsCloudProvider

	// assert the terminate function is called with the correct number of instances
	mockTerminateFunc := func(n *NodeGroup, i []*string) {
		assert.Equal(t, numInstances, len(i))
	}

	err := mockNodeGroup.attachInstancesToASG(instanceIDs, mockTerminateFunc)
	assert.Error(t, err)
}

func TestAttachInstancesToASG_NoSuccessfulBatches_ExpectFailure(t *testing.T) {
	setupAWSMocks()
	instanceID := "instanceID"

	terminateSize := 50
	numInstances := batchSize + terminateSize
	var instanceIDs []*string
	for i := 0; i < numInstances; i++ {
		instanceIDs = append(instanceIDs, &instanceID)
	}

	// Mock service call and error
	awsCloudProvider, _ := newMockCloudProviderUsingInjection(
		nil,
		&mockAutoscalingService{
			AutoScalingAPI: nil,
			Client:         nil,
			numCalls:       0,
			numMaxCalls:    0,
		},
		&test.MockEc2Service{
			TerminateInstancesOutput: &ec2.TerminateInstancesOutput{},
			TerminateInstancesErr:    nil,
			AllInstancesReady:        true,
		},
	)

	mockNodeGroup.provider = awsCloudProvider

	// assert the terminate function is called with the correct number of instances
	mockTerminateFunc := func(n *NodeGroup, i []*string) {
		assert.Equal(t, numInstances, len(i), "Expected all instances to be terminated")
	}

	err := mockNodeGroup.attachInstancesToASG(instanceIDs, mockTerminateFunc)
	assert.Error(t, err)
}

func TestAttachInstancesToASG_OneSuccessfulBatch_ExpectFailure(t *testing.T) {
	setupAWSMocks()
	instanceID := "instanceID"

	terminateSize := 50
	numInstances := batchSize + terminateSize
	var instanceIDs []*string
	for i := 0; i < numInstances; i++ {
		instanceIDs = append(instanceIDs, &instanceID)
	}

	// Mock service call and error
	awsCloudProvider, _ := newMockCloudProviderUsingInjection(
		nil,
		&mockAutoscalingService{
			AutoScalingAPI: nil,
			Client:         nil,
			numCalls:       0,
			numMaxCalls:    1,
		},
		&test.MockEc2Service{
			TerminateInstancesOutput: &ec2.TerminateInstancesOutput{},
			TerminateInstancesErr:    nil,
			AllInstancesReady:        true,
		},
	)

	mockNodeGroup.provider = awsCloudProvider

	// assert the terminate function is called with the correct number of instances
	mockTerminateFunc := func(n *NodeGroup, i []*string) {
		assert.Equal(t, terminateSize, len(i), "Expected all instances except the first batch to be terminated")
	}

	err := mockNodeGroup.attachInstancesToASG(instanceIDs, mockTerminateFunc)
	assert.Error(t, err)
}

func TestAttachInstancesToASG_NoBatches_ExpectFailure(t *testing.T) {
	setupAWSMocks()
	instanceID := "instanceID"

	terminateSize := batchSize - 1
	numInstances := terminateSize
	var instanceIDs []*string
	for i := 0; i < numInstances; i++ {
		instanceIDs = append(instanceIDs, &instanceID)
	}

	// Mock service call and error
	awsCloudProvider, _ := newMockCloudProviderUsingInjection(
		nil,
		&mockAutoscalingService{
			AutoScalingAPI: nil,
			Client:         nil,
			numCalls:       0,
			numMaxCalls:    0,
		},
		&test.MockEc2Service{
			TerminateInstancesOutput: &ec2.TerminateInstancesOutput{},
			TerminateInstancesErr:    nil,
			AllInstancesReady:        true,
		},
	)

	mockNodeGroup.provider = awsCloudProvider

	// assert the terminate function is called with the correct number of instances
	mockTerminateFunc := func(n *NodeGroup, i []*string) {
		assert.Equal(t, terminateSize, len(i))
	}

	err := mockNodeGroup.attachInstancesToASG(instanceIDs, mockTerminateFunc)
	assert.Error(t, err)
}

func TestAttachInstancesToASG_ExpectSuccess(t *testing.T) {
	setupAWSMocks()
	instanceID := "instanceID"

	terminateSize := batchSize - 1
	numInstances := terminateSize
	var instanceIDs []*string
	for i := 0; i < numInstances; i++ {
		instanceIDs = append(instanceIDs, &instanceID)
	}

	// Mock service call and error
	awsCloudProvider, _ := newMockCloudProviderUsingInjection(
		map[string]*NodeGroup{mockNodeGroup.id: &mockNodeGroup},
		&mockAutoscalingService{
			AutoScalingAPI: nil,
			Client:         nil,
			numCalls:       0,
			numMaxCalls:    1,
		},
		&test.MockEc2Service{
			TerminateInstancesOutput: &ec2.TerminateInstancesOutput{},
			TerminateInstancesErr:    nil,
			AllInstancesReady:        true,
		},
	)

	mockNodeGroup.provider = awsCloudProvider

	// the terminate function shouldn't get called - fail the test if it does
	mockTerminateFunc := func(n *NodeGroup, i []*string) {
		assert.Fail(t, "No instances should have been terminated")
	}

	err := mockNodeGroup.attachInstancesToASG(instanceIDs, mockTerminateFunc)
	assert.NoError(t, err)
}

func TestTerminateInstances_Success(t *testing.T) {
	setupAWSMocks()

	// Mock service call and error
	awsCloudProvider, _ := newMockCloudProviderUsingInjection(
		nil,
		&test.MockAutoscalingService{},
		&test.MockEc2Service{
			TerminateInstancesOutput: &ec2.TerminateInstancesOutput{},
			TerminateInstancesErr:    nil,
		},
	)

	instance1 := "i-123456"
	instances := []*string{&instance1}
	mockNodeGroup.provider = awsCloudProvider

	terminateOrphanedInstances(&mockNodeGroup, instances)
}

func TestTerminateInstances_WithErrorResponse(t *testing.T) {
	setupAWSMocks()

	// Mock service call and error
	awsCloudProvider, _ := newMockCloudProviderUsingInjection(
		nil,
		&test.MockAutoscalingService{},
		&test.MockEc2Service{
			TerminateInstancesOutput: &ec2.TerminateInstancesOutput{},
			TerminateInstancesErr:    errors.New("unauthorized"),
		},
	)

	instance1 := "i-123456"
	instances := []*string{&instance1}
	mockNodeGroup.provider = awsCloudProvider

	terminateOrphanedInstances(&mockNodeGroup, instances)
}

func TestMinInt(t *testing.T) {
	x := 1
	y := 2
	assert.Equal(t, x, minInt(x, y))
	assert.Equal(t, x, minInt(y, x))
	assert.Equal(t, x, minInt(x, x))
}
