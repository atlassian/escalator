package aws

import (
	"testing"

	"github.com/atlassian/escalator/pkg/test"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/stretchr/testify/assert"
)

func TestInstanceToProviderId(t *testing.T) {
	instance := &autoscaling.Instance{
		AvailabilityZone: aws.String("us-east-1b"),
		InstanceId:       aws.String("abc123"),
	}
	res := instanceToProviderId(instance)
	assert.Equal(t, "aws:///us-east-1b/abc123", res)
}

func TestProviderIdToInstanceId(t *testing.T) {
	assert.Equal(t, "abc123", providerIdToInstanceId("aws:///us-east-1b/abc123"))
}

func newMockCloudProvider(nodeGroups []string, service *test.MockAutoscalingService, ec2_service *test.MockEc2Service) (*CloudProvider, error) {
	var err error

	cloudProvider := &CloudProvider{
		service:     service,
		ec2_service: ec2_service,
		nodeGroups:  make(map[string]*NodeGroup, len(nodeGroups)),
	}

	if service != nil {
		err = cloudProvider.RegisterNodeGroups(nodeGroups...)
	}

	return cloudProvider, err
}
