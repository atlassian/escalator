package aws

import (
	"errors"
	"testing"
	"time"

	"github.com/atlassian/escalator/pkg/cloudprovider"
	"github.com/atlassian/escalator/pkg/test"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
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
	}

	mockAWSConfig = cloudprovider.AWSNodeGroupConfig{
		LaunchTemplateID:          "lt-123456",
		LaunchTemplateVersion:     "1",
		FleetInstanceReadyTimeout: tickerTimeout,
		Lifecycle:                 LifecycleOnDemand,
		InstanceTypeOverrides:     []string{"instance-1", "instance-2"},
	}

	mockNodeGroup = NodeGroup{
		id:     "id",
		name:   "name",
		config: &mockNodeGroupConfig,
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
func newMockCloudProviderUsingInjection(nodeGroups map[string]*NodeGroup, service *test.MockAutoscalingService, ec2Service *test.MockEc2Service) (*CloudProvider, error) {
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
