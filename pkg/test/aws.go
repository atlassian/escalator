package test

import (
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
)

type MockAutoscalingService struct {
	autoscalingiface.AutoScalingAPI
	*client.Client

	DescribeAutoScalingGroupsOutput *autoscaling.DescribeAutoScalingGroupsOutput
	DescribeAutosCalingGroupsErr    error
}

func (m MockAutoscalingService) DescribeAutoScalingGroups(*autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	return m.DescribeAutoScalingGroupsOutput, m.DescribeAutosCalingGroupsErr
}
