package test

import (
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

// MockAutoscalingService is a mock implementation of a cloud provider interface
type MockAutoscalingService struct {
	autoscalingiface.AutoScalingAPI
	*client.Client

	AttachInstanceOutput *autoscaling.AttachInstancesOutput
	AttachInstanceErr    error

	CreateOrUpdateTagsOutput *autoscaling.CreateOrUpdateTagsOutput
	CreateOrUpdateTagsErr    error

	DescribeAutoScalingGroupsOutput *autoscaling.DescribeAutoScalingGroupsOutput
	DescribeAutoScalingGroupsErr    error

	SetDesiredCapacityOutput *autoscaling.SetDesiredCapacityOutput
	SetDesiredCapacityErr    error

	TerminateInstanceInAutoScalingGroupOutput *autoscaling.TerminateInstanceInAutoScalingGroupOutput
	TerminateInstanceInAutoScalingGroupErr    error
}

// AttachInstances mock implementation for MockAutoscalingService
func (m MockAutoscalingService) AttachInstances(*autoscaling.AttachInstancesInput) (*autoscaling.AttachInstancesOutput, error) {
	return m.AttachInstanceOutput, m.AttachInstanceErr
}

// CreateOrUpdateTags mock implementation for MockAutoscalingService
func (m MockAutoscalingService) CreateOrUpdateTags(*autoscaling.CreateOrUpdateTagsInput) (*autoscaling.CreateOrUpdateTagsOutput, error) {
	return m.CreateOrUpdateTagsOutput, m.CreateOrUpdateTagsErr
}

// DescribeAutoScalingGroups mock implementation for MockAutoscalingService
func (m MockAutoscalingService) DescribeAutoScalingGroups(*autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	return m.DescribeAutoScalingGroupsOutput, m.DescribeAutoScalingGroupsErr
}

// SetDesiredCapacity mock implementation for MockAutoscalingService
func (m MockAutoscalingService) SetDesiredCapacity(*autoscaling.SetDesiredCapacityInput) (*autoscaling.SetDesiredCapacityOutput, error) {
	return m.SetDesiredCapacityOutput, m.SetDesiredCapacityErr
}

// TerminateInstanceInAutoScalingGroup mock implementation for MockAutoscalingService
func (m MockAutoscalingService) TerminateInstanceInAutoScalingGroup(*autoscaling.TerminateInstanceInAutoScalingGroupInput) (*autoscaling.TerminateInstanceInAutoScalingGroupOutput, error) {
	return m.TerminateInstanceInAutoScalingGroupOutput, m.TerminateInstanceInAutoScalingGroupErr
}

// MockEc2Service mocks the EC2API
type MockEc2Service struct {
	ec2iface.EC2API
	*client.Client

	CreateFleetOutput *ec2.CreateFleetOutput
	CreateFleetErr    error

	DescribeInstancesOutput *ec2.DescribeInstancesOutput
	DescribeInstancesErr    error

	DescribeInstanceStatusOutput *ec2.DescribeInstanceStatusOutput
	DescribeInstanceStatusErr    error
	AllInstancesReady            bool

	TerminateInstancesOutput *ec2.TerminateInstancesOutput
	TerminateInstancesErr    error
}

// DescribeInstances mock implementation for MockEc2Service
func (m MockEc2Service) DescribeInstances(*ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	return m.DescribeInstancesOutput, m.DescribeInstancesErr
}

// CreateFleet mock implementation for MockEc2Service
func (m MockEc2Service) CreateFleet(*ec2.CreateFleetInput) (*ec2.CreateFleetOutput, error) {
	return m.CreateFleetOutput, m.CreateFleetErr
}

// DescribeInstanceStatusPages mock implementation for MockEc2Service
func (m MockEc2Service) DescribeInstanceStatusPages(statusInput *ec2.DescribeInstanceStatusInput, allInstancesReadyHelper func(*ec2.DescribeInstanceStatusOutput, bool) bool) error {
	// Mocks execution of the anonymous function within cloudprovider/aws/aws.go:allInstancesReady
	allInstancesReadyHelper(&ec2.DescribeInstanceStatusOutput{}, m.AllInstancesReady)
	return nil
}

// TerminateInstances mock implementation for MockEc2Service
func (m MockEc2Service) TerminateInstances(*ec2.TerminateInstancesInput) (*ec2.TerminateInstancesOutput, error) {
	return m.TerminateInstancesOutput, m.TerminateInstancesErr
}
