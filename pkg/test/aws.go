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
	DescribeAutoScalingGroupsErr    error

	SetDesiredCapacityOutput *autoscaling.SetDesiredCapacityOutput
	SetDesiredCapacityErr    error

	TerminateInstanceInAutoScalingGroupOutput *autoscaling.TerminateInstanceInAutoScalingGroupOutput
	TerminateInstanceInAutoScalingGroupErr    error
}

func (m MockAutoscalingService) DescribeAutoScalingGroups(*autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	return m.DescribeAutoScalingGroupsOutput, m.DescribeAutoScalingGroupsErr
}

func (m MockAutoscalingService) SetDesiredCapacity(*autoscaling.SetDesiredCapacityInput) (*autoscaling.SetDesiredCapacityOutput, error) {
	return m.SetDesiredCapacityOutput, m.SetDesiredCapacityErr
}

func (m MockAutoscalingService) TerminateInstanceInAutoScalingGroup(*autoscaling.TerminateInstanceInAutoScalingGroupInput) (*autoscaling.TerminateInstanceInAutoScalingGroupOutput, error) {
	return m.TerminateInstanceInAutoScalingGroupOutput, m.TerminateInstanceInAutoScalingGroupErr
}
