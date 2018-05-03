package aws

import (
	"fmt"
	"github.com/atlassian/escalator/pkg/test"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/stretchr/testify/assert"
	"testing"
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
				"1": NewNodeGroup("1", &autoscaling.Group{}, &CloudProvider{}),
			},
		},
		{
			"multiple node groups",
			map[string]*NodeGroup{
				"1": NewNodeGroup("1", &autoscaling.Group{}, &CloudProvider{}),
				"2": NewNodeGroup("2", &autoscaling.Group{}, &CloudProvider{}),
			},
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
				"1": NewNodeGroup("1", &autoscaling.Group{}, &CloudProvider{}),
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
		name       string
		nodeGroups map[string]bool
		response   *autoscaling.DescribeAutoScalingGroupsOutput
		err        error
	}{
		{
			"register node group that does not exist",
			map[string]bool{
				"1": false,
			},
			&autoscaling.DescribeAutoScalingGroupsOutput{},
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
		},
		{
			"register no node groups",
			map[string]bool{},
			&autoscaling.DescribeAutoScalingGroupsOutput{},
			fmt.Errorf("no groups"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock autoscaling service
			service := test.MockAutoscalingService{
				DescribeAutoScalingGroupsOutput: tt.response,
				DescribeAutoScalingGroupsErr:    tt.err,
			}

			var ids []string
			for id := range tt.nodeGroups {
				ids = append(ids, id)
			}

			awsCloudProvider, err := newMockCloudProvider(ids, &service)
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
	})
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
