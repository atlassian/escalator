package aws

import (
	"math/rand"
	"testing"

	"github.com/atlassian/escalator/pkg/cloudprovider"
	"github.com/atlassian/escalator/pkg/test"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNodeGroup_ID(t *testing.T) {
	id := "nodegroup"
	nodeGroup := NewNodeGroup(&cloudprovider.NodeGroupConfig{GroupID: id}, &autoscaling.Group{}, &CloudProvider{})
	assert.Equal(t, id, nodeGroup.ID())
}

func TestNodeGroup_String(t *testing.T) {
	nodeGroup := NewNodeGroup(&cloudprovider.NodeGroupConfig{GroupID: "nodegroup"}, &autoscaling.Group{}, &CloudProvider{})
	assert.IsType(t, "string", nodeGroup.String())
}

func TestNodeGroup_MinSize(t *testing.T) {
	minSize := rand.Int63()

	asg := &autoscaling.Group{
		MinSize: aws.Int64(minSize),
	}

	nodeGroup := NewNodeGroup(&cloudprovider.NodeGroupConfig{GroupID: "nodegroup"}, asg, &CloudProvider{})
	assert.Equal(t, minSize, nodeGroup.MinSize())
}

func TestNodeGroup_MaxSize(t *testing.T) {
	maxSize := rand.Int63()

	asg := &autoscaling.Group{
		MaxSize: aws.Int64(maxSize),
	}

	nodeGroup := NewNodeGroup(&cloudprovider.NodeGroupConfig{GroupID: "nodegroup"}, asg, &CloudProvider{})
	assert.Equal(t, maxSize, nodeGroup.MaxSize())
}

func TestNodeGroup_TargetSize(t *testing.T) {
	id := "nodegroup"
	desiredCapacity := rand.Int63()

	asg := &autoscaling.Group{
		DesiredCapacity: aws.Int64(desiredCapacity),
	}

	nodeGroup := NewNodeGroup(&cloudprovider.NodeGroupConfig{GroupID: id}, asg, &CloudProvider{})
	assert.Equal(t, desiredCapacity, nodeGroup.TargetSize())
}

func TestNodeGroup_Size(t *testing.T) {
	tests := []struct {
		name      string
		instances []*autoscaling.Instance
		expected  int64
	}{
		{
			"multiple instances",
			[]*autoscaling.Instance{
				{InstanceId: aws.String("1")},
				{InstanceId: aws.String("2")},
				{InstanceId: aws.String("3")},
			},
			int64(3),
		},
		{
			"no instances",
			[]*autoscaling.Instance{},
			int64(0),
		},
		{
			"one instance",
			[]*autoscaling.Instance{
				{InstanceId: aws.String("1")},
			},
			int64(1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asg := &autoscaling.Group{Instances: tt.instances}
			nodeGroup := NewNodeGroup(&cloudprovider.NodeGroupConfig{GroupID: "nodegroup"}, asg, &CloudProvider{})
			assert.Equal(t, tt.expected, nodeGroup.Size())
		})
	}
}

func TestNodeGroup_IncreaseSize(t *testing.T) {
	tests := []struct {
		name              string
		increaseSize      int64
		autoScalingGroups []*autoscaling.Group
		err               error
	}{
		{
			"normal increase",
			int64(5),
			[]*autoscaling.Group{
				{
					AutoScalingGroupName: aws.String("asg-1"),
					MaxSize:              aws.Int64(int64(10)),
					DesiredCapacity:      aws.Int64(int64(1)),
				},
			},
			nil,
		},
		{
			"negative increase",
			int64(-1),
			[]*autoscaling.Group{
				{
					AutoScalingGroupName: aws.String("asg-2"),
					MaxSize:              aws.Int64(int64(10)),
					DesiredCapacity:      aws.Int64(int64(1)),
				},
			},
			errors.New("size increase must be positive"),
		},
		{
			"zero increase",
			int64(0),
			[]*autoscaling.Group{
				{
					AutoScalingGroupName: aws.String("asg-3"),
					MaxSize:              aws.Int64(int64(10)),
					DesiredCapacity:      aws.Int64(int64(1)),
				},
			},
			errors.New("size increase must be positive"),
		},
		{
			"breach max size increase",
			int64(20),
			[]*autoscaling.Group{
				{
					AutoScalingGroupName: aws.String("asg-4"),
					MaxSize:              aws.Int64(int64(10)),
					DesiredCapacity:      aws.Int64(int64(1)),
				},
			},
			errors.New("increasing size will breach maximum node size"),
		},
	}

	// Run tests with ASG scaling
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var nodeGroupNames []string
			for _, asg := range tt.autoScalingGroups {
				nodeGroupNames = append(nodeGroupNames, aws.StringValue(asg.AutoScalingGroupName))
			}

			awsCloudProvider, err := newMockCloudProvider(nodeGroupNames, &test.MockAutoscalingService{
				DescribeAutoScalingGroupsOutput: &autoscaling.DescribeAutoScalingGroupsOutput{
					AutoScalingGroups: tt.autoScalingGroups,
				},
			},
				nil)
			assert.Nil(t, err)

			for _, nodeGroup := range awsCloudProvider.NodeGroups() {
				err = nodeGroup.IncreaseSize(tt.increaseSize)
				if tt.err == nil {
					require.NoError(t, err)
				} else {
					require.EqualError(t, tt.err, err.Error())
				}
			}
		})
	}
}

func TestNodeGroup_IncreaseSize_CreateFleet(t *testing.T) {
	setupAWSMocks()
	fleetID := "fleet-1234"
	errorCode := "error code"
	errorMessage := "error message"
	instanceID := "instanceID"
	multipleBatches := batchSize + 1
	multipleBatchesInstanceIDs := make([]*string, multipleBatches)
	for i := 0; i < multipleBatches; i++ {
		multipleBatchesInstanceIDs = append(multipleBatchesInstanceIDs, &instanceID)
	}

	createFleetTests := []struct {
		name              string
		increaseSize      int64
		createFleetOutput *ec2.CreateFleetOutput
		err               error
	}{
		{
			"normal increase",
			int64(1),
			&ec2.CreateFleetOutput{
				Errors:  nil,
				FleetId: &fleetID,
				Instances: []*ec2.CreateFleetInstance{
					{
						InstanceIds: []*string{&instanceID},
					},
				},
			},
			nil,
		},
		{
			"normal increase with error",
			int64(1),
			&ec2.CreateFleetOutput{
				Errors: []*ec2.CreateFleetError{
					{
						LaunchTemplateAndOverrides: nil,
						Lifecycle:                  nil,
						ErrorCode:                  &errorCode,
						ErrorMessage:               &errorMessage,
					},
				},
				FleetId: &fleetID,
				Instances: []*ec2.CreateFleetInstance{
					{
						InstanceIds: []*string{&instanceID},
					},
				},
			},
			nil,
		},
		{
			"no instances error",
			int64(5),
			&ec2.CreateFleetOutput{
				Errors: []*ec2.CreateFleetError{
					{
						LaunchTemplateAndOverrides: nil,
						Lifecycle:                  nil,
						ErrorCode:                  &errorCode,
						ErrorMessage:               &errorMessage,
					},
				},
				FleetId:   &fleetID,
				Instances: make([]*ec2.CreateFleetInstance, 0),
			},
			errors.New(errorMessage),
		},
		{
			"multiple AttachInstances batches",
			int64(multipleBatches),
			&ec2.CreateFleetOutput{
				Errors:  nil,
				FleetId: &fleetID,
				Instances: []*ec2.CreateFleetInstance{
					{
						InstanceIds: multipleBatchesInstanceIDs,
					},
				},
			},
			nil,
		},
	}

	// Necessary for setting up node group
	autoScalingGroup := []*autoscaling.Group{&mockASG}

	// Run tests with CreateFleet scaling
	for _, tt := range createFleetTests {
		t.Run(tt.name, func(t *testing.T) {
			nodeGroup := NodeGroup{
				id:     "id",
				name:   "name",
				asg:    autoScalingGroup[0],
				config: &mockNodeGroupConfig,
			}
			nodeGroups := map[string]*NodeGroup{nodeGroup.id: &nodeGroup}

			awsCloudProvider, err := newMockCloudProviderUsingInjection(
				nodeGroups,
				&test.MockAutoscalingService{
					DescribeAutoScalingGroupsOutput: &autoscaling.DescribeAutoScalingGroupsOutput{
						AutoScalingGroups: autoScalingGroup,
					},
				},
				&test.MockEc2Service{
					CreateFleetOutput:       tt.createFleetOutput,
					DescribeInstancesOutput: &ec2.DescribeInstancesOutput{},
					AllInstancesReady:       true,
				},
			)
			require.NoError(t, err)

			for _, nodeGroup := range awsCloudProvider.NodeGroups() {
				err = nodeGroup.IncreaseSize(tt.increaseSize)
				if tt.err == nil {
					require.NoError(t, err)
				} else {
					require.EqualError(t, tt.err, err.Error())
				}
			}
		})
	}
}

func TestNodeGroup_DeleteNodes(t *testing.T) {
	type group struct {
		asg                                       *autoscaling.Group
		nodesToDelete                             []*v1.Node
		terminateInstanceInAutoScalingGroupOutput *autoscaling.TerminateInstanceInAutoScalingGroupOutput
		terminateInstanceInAutoScalingGroupErr    error
		err                                       error
	}

	tests := []struct {
		name   string
		groups []*group
	}{
		{
			"delete existing nodes",
			[]*group{
				{
					&autoscaling.Group{
						AutoScalingGroupName: aws.String("asg-1"),
						MinSize:              aws.Int64(int64(1)),
						MaxSize:              aws.Int64(int64(10)),
						DesiredCapacity:      aws.Int64(int64(4)),
						Instances: []*autoscaling.Instance{
							{InstanceId: aws.String("instance-1"), AvailabilityZone: aws.String("us-east-1a")},
							{InstanceId: aws.String("instance-2"), AvailabilityZone: aws.String("us-east-1a")},
							{InstanceId: aws.String("instance-3"), AvailabilityZone: aws.String("us-east-1a")},
							{InstanceId: aws.String("instance-4"), AvailabilityZone: aws.String("us-east-1a")},
						},
					},
					[]*v1.Node{
						{
							Spec: v1.NodeSpec{
								ProviderID: "aws:///us-east-1a/instance-2",
							},
						},
						{
							Spec: v1.NodeSpec{
								ProviderID: "aws:///us-east-1a/instance-3",
							},
						},
					},
					&autoscaling.TerminateInstanceInAutoScalingGroupOutput{
						Activity: &autoscaling.Activity{
							Description: aws.String("successfully terminated instance"),
						},
					},
					nil,
					nil,
				},
			},
		},
		{
			"breach minimum size",
			[]*group{
				{
					&autoscaling.Group{
						AutoScalingGroupName: aws.String("asg-1"),
						MinSize:              aws.Int64(int64(1)),
						MaxSize:              aws.Int64(int64(10)),
						DesiredCapacity:      aws.Int64(int64(2)),
						Instances: []*autoscaling.Instance{
							{InstanceId: aws.String("instance-1"), AvailabilityZone: aws.String("us-east-1a")},
							{InstanceId: aws.String("instance-2"), AvailabilityZone: aws.String("us-east-1a")},
						},
					},
					[]*v1.Node{
						{
							Spec: v1.NodeSpec{
								ProviderID: "aws:///us-east-1a/instance-1",
							},
						},
						{
							Spec: v1.NodeSpec{
								ProviderID: "aws:///us-east-1a/instance-2",
							},
						},
					},

					&autoscaling.TerminateInstanceInAutoScalingGroupOutput{},
					nil,
					errors.New("terminating nodes will breach minimum node size"),
				},
			},
		},
		{
			"already at minimum size",
			[]*group{
				{
					&autoscaling.Group{
						AutoScalingGroupName: aws.String("asg-1"),
						MinSize:              aws.Int64(int64(1)),
						MaxSize:              aws.Int64(int64(10)),
						DesiredCapacity:      aws.Int64(int64(1)),
						Instances: []*autoscaling.Instance{
							{InstanceId: aws.String("instance-1"), AvailabilityZone: aws.String("us-east-1a")},
						},
					},
					[]*v1.Node{
						{
							Spec: v1.NodeSpec{
								ProviderID: "aws:///us-east-1a/instance-1",
							},
						},
					},
					&autoscaling.TerminateInstanceInAutoScalingGroupOutput{},
					nil,
					errors.New("min sized reached, nodes will not be deleted"),
				},
			},
		},
		{
			"delete non-existent node",
			[]*group{
				{
					&autoscaling.Group{
						AutoScalingGroupName: aws.String("asg-1"),
						MinSize:              aws.Int64(int64(1)),
						MaxSize:              aws.Int64(int64(10)),
						DesiredCapacity:      aws.Int64(int64(2)),
						Instances: []*autoscaling.Instance{
							{InstanceId: aws.String("instance-1"), AvailabilityZone: aws.String("us-east-1a")},
							{InstanceId: aws.String("instance-2"), AvailabilityZone: aws.String("us-east-1a")},
						},
					},
					[]*v1.Node{
						{
							ObjectMeta: metaV1.ObjectMeta{
								Name: "instance-3",
							},
							Spec: v1.NodeSpec{
								ProviderID: "aws:///us-east-1a/instance-3",
							},
						},
					},
					&autoscaling.TerminateInstanceInAutoScalingGroupOutput{},
					nil,
					&cloudprovider.NodeNotInNodeGroup{NodeName: "instance-3", ProviderID: "aws:///us-east-1a/instance-3", NodeGroup: "asg-1"},
				},
			},
		},
		{
			"terminate instance error",
			[]*group{
				{
					&autoscaling.Group{
						AutoScalingGroupName: aws.String("asg-1"),
						MinSize:              aws.Int64(int64(1)),
						MaxSize:              aws.Int64(int64(10)),
						DesiredCapacity:      aws.Int64(int64(2)),
						Instances: []*autoscaling.Instance{
							{InstanceId: aws.String("instance-1"), AvailabilityZone: aws.String("us-east-1a")},
							{InstanceId: aws.String("instance-2"), AvailabilityZone: aws.String("us-east-1a")},
						},
					},
					[]*v1.Node{
						{
							ObjectMeta: metaV1.ObjectMeta{
								Name: "instance-2",
							},
							Spec: v1.NodeSpec{
								ProviderID: "aws:///us-east-1a/instance-2",
							},
						},
					},
					&autoscaling.TerminateInstanceInAutoScalingGroupOutput{},
					errors.New("unable to terminate instance"),
					errors.New("failed to terminate instance. err: unable to terminate instance"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Aggregate all of the node group names and autoscaling groups
			var nodeGroupNames []string
			var autoscalingGroups []*autoscaling.Group
			for _, group := range tt.groups {
				nodeGroupNames = append(nodeGroupNames, aws.StringValue(group.asg.AutoScalingGroupName))
				autoscalingGroups = append(autoscalingGroups, group.asg)
			}

			// Create the mock autoscaling service
			mockAutoScalingService := test.MockAutoscalingService{
				DescribeAutoScalingGroupsOutput: &autoscaling.DescribeAutoScalingGroupsOutput{
					AutoScalingGroups: autoscalingGroups,
				},
			}

			// Create the aws cloud provider
			awsCloudProvider, err := newMockCloudProvider(nodeGroupNames, &mockAutoScalingService, nil)
			assert.Nil(t, err)

			// Delete nodes from each node group
			for _, group := range tt.groups {
				name := aws.StringValue(group.asg.AutoScalingGroupName)
				nodeGroup, ok := awsCloudProvider.GetNodeGroup(name)
				assert.True(t, ok)

				// Terminate the instances
				mockAutoScalingService.TerminateInstanceInAutoScalingGroupOutput = group.terminateInstanceInAutoScalingGroupOutput
				mockAutoScalingService.TerminateInstanceInAutoScalingGroupErr = group.terminateInstanceInAutoScalingGroupErr
				err := nodeGroup.DeleteNodes(group.nodesToDelete...)
				if group.err == nil {
					require.NoError(t, err)
				} else {
					require.EqualError(t, group.err, err.Error())
				}
			}
		})
	}
}

func TestNodeGroup_DecreaseSize(t *testing.T) {
	tests := []struct {
		name              string
		decreaseSize      int64
		autoScalingGroups []*autoscaling.Group
		err               error
	}{
		{
			"normal decrease",
			int64(-5),
			[]*autoscaling.Group{
				{
					AutoScalingGroupName: aws.String("asg-1"),
					MinSize:              aws.Int64(int64(1)),
					DesiredCapacity:      aws.Int64(int64(10)),
				},
			},
			nil,
		},
		{
			"positive decrease",
			int64(5),
			[]*autoscaling.Group{
				{
					AutoScalingGroupName: aws.String("asg-2"),
					MinSize:              aws.Int64(int64(1)),
					DesiredCapacity:      aws.Int64(int64(10)),
				},
			},
			errors.New("size decrease delta must be negative"),
		},
		{
			"zero decrease",
			int64(0),
			[]*autoscaling.Group{
				{
					AutoScalingGroupName: aws.String("asg-3"),
					MinSize:              aws.Int64(int64(0)),
					DesiredCapacity:      aws.Int64(int64(10)),
				},
			},
			errors.New("size decrease delta must be negative"),
		},
		{
			"breach min size decrease",
			int64(-20),
			[]*autoscaling.Group{
				{
					AutoScalingGroupName: aws.String("asg-4"),
					MinSize:              aws.Int64(int64(1)),
					DesiredCapacity:      aws.Int64(int64(10)),
				},
			},
			errors.New("decreasing target size will breach minimum node size"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var nodeGroupNames []string
			for _, asg := range tt.autoScalingGroups {
				nodeGroupNames = append(nodeGroupNames, aws.StringValue(asg.AutoScalingGroupName))
			}

			awsCloudProvider, err := newMockCloudProvider(nodeGroupNames, &test.MockAutoscalingService{
				DescribeAutoScalingGroupsOutput: &autoscaling.DescribeAutoScalingGroupsOutput{
					AutoScalingGroups: tt.autoScalingGroups,
				},
			},
				nil)
			assert.Nil(t, err)

			for _, nodeGroup := range awsCloudProvider.NodeGroups() {
				err = nodeGroup.DecreaseTargetSize(tt.decreaseSize)
				if tt.err == nil {
					require.NoError(t, err)
				} else {
					require.EqualError(t, tt.err, err.Error())
				}
			}
		})
	}
}
