package aws

import (
	"fmt"
	"testing"

	"github.com/atlassian/escalator/pkg/cloudprovider"
	"github.com/atlassian/escalator/pkg/test"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

func TestCloudProvider_Name(t *testing.T) {
	awsCloudProvider := &CloudProvider{}
	assert.Equal(t, ProviderName, awsCloudProvider.Name())
}

func TestCloudProvider_NodeGroups(t *testing.T) {
	tests := []struct {
		name       string
		nodeGroups map[string]*NodeGroup
	}{
		{
			"single node group",
			map[string]*NodeGroup{
				"1": NewNodeGroup(&cloudprovider.NodeGroupConfig{GroupID: "1"}, &autoscaling.Group{}, &CloudProvider{}),
			},
		},
		{
			"multiple node groups",
			map[string]*NodeGroup{
				"1": NewNodeGroup(&cloudprovider.NodeGroupConfig{GroupID: "1"}, &autoscaling.Group{}, &CloudProvider{}),
				"2": NewNodeGroup(&cloudprovider.NodeGroupConfig{GroupID: "2"}, &autoscaling.Group{}, &CloudProvider{})},
		},
		{
			"no node groups",
			map[string]*NodeGroup{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			awsCloudProvider := &CloudProvider{
				nodeGroups: tt.nodeGroups,
			}
			assert.Len(t, awsCloudProvider.NodeGroups(), len(tt.nodeGroups))
		})
	}
}

func TestCloudProvider_GetNodeGroup(t *testing.T) {
	tests := []struct {
		name       string
		nodeGroups map[string]*NodeGroup
		id         string
		ok         bool
	}{
		{
			"get a node group that exists",
			map[string]*NodeGroup{
				"1": NewNodeGroup(&cloudprovider.NodeGroupConfig{GroupID: "1"}, &autoscaling.Group{}, &CloudProvider{}),
			},
			"1",
			true,
		},
		{
			"get a node group that does not exist",
			map[string]*NodeGroup{},
			"1",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			awsCloudProvider := &CloudProvider{
				nodeGroups: tt.nodeGroups,
			}

			res, ok := awsCloudProvider.GetNodeGroup(tt.id)
			assert.Equal(t, tt.ok, ok)
			if ok {
				assert.Equal(t, tt.nodeGroups[tt.id], res)
			}
		})
	}
}

func TestCloudProvider_RegisterNodeGroups(t *testing.T) {
	tests := []struct {
		name        string
		nodeGroups  map[string]bool
		response    *autoscaling.DescribeAutoScalingGroupsOutput
		err         error
		tagResponse *autoscaling.CreateOrUpdateTagsOutput
		tagErr      error
	}{
		{
			"register node group that does not exist",
			map[string]bool{
				"1": false,
			},
			&autoscaling.DescribeAutoScalingGroupsOutput{},
			nil,
			&autoscaling.CreateOrUpdateTagsOutput{},
			nil,
		},
		{
			"register node groups that exist",
			map[string]bool{
				"1": true,
				"2": true,
			},
			&autoscaling.DescribeAutoScalingGroupsOutput{
				AutoScalingGroups: []*autoscaling.Group{
					{
						AutoScalingGroupName: aws.String("1"),
					},
					{
						AutoScalingGroupName: aws.String("2"),
					},
				},
			},
			nil,
			&autoscaling.CreateOrUpdateTagsOutput{},
			nil,
		},
		{
			"register node groups, some don't exist",
			map[string]bool{
				"1": true,
				"2": false,
			},
			&autoscaling.DescribeAutoScalingGroupsOutput{
				AutoScalingGroups: []*autoscaling.Group{
					{
						AutoScalingGroupName: aws.String("1"),
					},
				},
			},
			nil,
			&autoscaling.CreateOrUpdateTagsOutput{},
			nil,
		},
		{
			"register no node groups",
			map[string]bool{},
			&autoscaling.DescribeAutoScalingGroupsOutput{},
			fmt.Errorf("no groups"),
			&autoscaling.CreateOrUpdateTagsOutput{},
			nil,
		},
		{
			"register existing node group with error from CreateOrUpdateTags",
			map[string]bool{
				"1": true,
			},
			&autoscaling.DescribeAutoScalingGroupsOutput{
				AutoScalingGroups: []*autoscaling.Group{
					{
						AutoScalingGroupName: aws.String("1"),
					},
				},
			},
			nil,
			&autoscaling.CreateOrUpdateTagsOutput{},
			fmt.Errorf("unauthorized operation"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock autoscaling service
			service := test.MockAutoscalingService{
				DescribeAutoScalingGroupsOutput: tt.response,
				DescribeAutoScalingGroupsErr:    tt.err,
				CreateOrUpdateTagsOutput:        tt.tagResponse,
				CreateOrUpdateTagsErr:           tt.tagErr,
			}

			var ids []string
			for id := range tt.nodeGroups {
				ids = append(ids, id)
			}

			awsCloudProvider, err := newMockCloudProvider(ids, &service, nil)
			assert.Equal(t, tt.err, err)

			// Ensure that the node groups that are supposed to exist have been fetched and registered properly
			for id, exists := range tt.nodeGroups {
				nodeGroup, ok := awsCloudProvider.GetNodeGroup(id)
				assert.Equal(t, exists, ok)
				if ok {
					assert.Equal(t, id, nodeGroup.ID())
				}
			}
		})
	}
}

func TestCloudProvider_Refresh(t *testing.T) {
	nodeGroups := []string{"1"}
	initialDesiredCapacity := int64(1)
	updatedDesiredCapacity := int64(2)

	// Create the autoscaling groups output
	var autoscalingGroups []*autoscaling.Group
	for _, id := range nodeGroups {
		autoscalingGroups = append(autoscalingGroups, &autoscaling.Group{
			AutoScalingGroupName: aws.String(id),
			DesiredCapacity:      aws.Int64(initialDesiredCapacity),
		})
	}

	// Create the initial response
	resp := &autoscaling.DescribeAutoScalingGroupsOutput{AutoScalingGroups: autoscalingGroups}

	awsCloudProvider, err := newMockCloudProvider(nodeGroups, &test.MockAutoscalingService{
		DescribeAutoScalingGroupsOutput: resp,
		DescribeAutoScalingGroupsErr:    nil,
	},
		nil)
	assert.Nil(t, err)

	// Ensure the node group is registered
	for _, id := range nodeGroups {
		nodeGroup, ok := awsCloudProvider.GetNodeGroup(id)
		assert.True(t, ok)
		assert.Equal(t, id, nodeGroup.ID())
		assert.Equal(t, initialDesiredCapacity, nodeGroup.TargetSize())
	}

	// Update the response
	for i := range nodeGroups {
		resp.AutoScalingGroups[i].DesiredCapacity = aws.Int64(updatedDesiredCapacity)
	}

	// Refresh the cloud provider
	err = awsCloudProvider.Refresh()
	assert.Nil(t, err)

	// Ensure the node group has been refreshed
	for _, id := range nodeGroups {
		nodeGroup, ok := awsCloudProvider.GetNodeGroup(id)
		assert.True(t, ok)
		assert.Equal(t, id, nodeGroup.ID())
		assert.Equal(t, updatedDesiredCapacity, nodeGroup.TargetSize())
	}
}

func TestCloudProvider_GetInstance(t *testing.T) {
	tests := []struct {
		name     string
		response *ec2.DescribeInstancesOutput
		err      error
	}{
		{
			"error describing instances",
			nil,
			fmt.Errorf("I like π"),
		},
		{
			"error in reservation count",
			&ec2.DescribeInstancesOutput{},
			fmt.Errorf("Malformed DescribeInstances response from AWS, expected only 1 Reservation and 1 Instance for id: abc123"),
		},
		{
			"error in instances count",
			&ec2.DescribeInstancesOutput{Reservations: []*ec2.Reservation{}},
			fmt.Errorf("Malformed DescribeInstances response from AWS, expected only 1 Reservation and 1 Instance for id: abc123"),
		},
		{
			"successful retrieval",
			&ec2.DescribeInstancesOutput{Reservations: []*ec2.Reservation{{Instances: []*ec2.Instance{{}}}}},
			nil,
		},
	}

	nodeGroups := []string{"1"}
	node := &v1.Node{
		Spec: v1.NodeSpec{
			ProviderID: "aws:///us-east-2b/abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock ec2 service
			ec2Service := &test.MockEc2Service{
				DescribeInstancesOutput: tt.response,
				DescribeInstancesErr:    tt.err,
			}

			awsCloudProvider, err := newMockCloudProvider(nodeGroups, nil, ec2Service)

			assert.Nil(t, err)

			instance, err := awsCloudProvider.GetInstance(node)
			assert.Equal(t, tt.err, err)
			if tt.err != nil {
				assert.Nil(t, instance)
			} else {
				assert.NotNil(t, instance)
			}
		})
	}
}
