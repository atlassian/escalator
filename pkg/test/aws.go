package test

import (
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
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

type MockEc2Service struct {
	ec2iface.EC2API
	*client.Client

	DescribeInstancesOutput *ec2.DescribeInstancesOutput
	DescribeInstancesErr    error
}

func (m MockEc2Service) DescribeInstances(*ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	return m.DescribeInstancesOutput, m.DescribeInstancesErr
}
