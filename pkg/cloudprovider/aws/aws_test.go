package aws

import (
	"github.com/atlassian/escalator/pkg/test"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestInstanceToProviderID(t *testing.T) {
	instance := &autoscaling.Instance{
		AvailabilityZone: aws.String("us-east-1b"),
		InstanceId:       aws.String("abc123"),
	}
	res := instanceToProviderID(instance)
	assert.Equal(t, "aws:///us-east-1b/abc123", res)
}

func newMockCloudProvider(nodeGroups []string, service test.MockAutoscalingService) (*CloudProvider, error) {
	cloudProvider := &CloudProvider{
		service:    service,
		nodeGroups: make(map[string]*NodeGroup, len(nodeGroups)),
	}
	return cloudProvider, cloudProvider.RegisterNodeGroups(nodeGroups...)
}
