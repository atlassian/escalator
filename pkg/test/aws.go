package test

import (
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
)

type AutoscalingMockedDescribeAutoScalingGroups struct {
	autoscalingiface.AutoScalingAPI
	*client.Client

	Resp  autoscaling.DescribeAutoScalingGroupsOutput
	Error error
}

func (a AutoscalingMockedDescribeAutoScalingGroups) DescribeAutoScalingGroups(*autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	return &a.Resp, a.Error
}
