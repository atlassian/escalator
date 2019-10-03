package aws

import (
	"testing"

	"github.com/atlassian/escalator/pkg/cloudprovider"
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
	res := instanceToProviderID(instance)
	assert.Equal(t, "aws:///us-east-1b/abc123", res)
}

func TestProviderIdToInstanceId(t *testing.T) {
	assert.Equal(t, "abc123", providerIDToInstanceID("aws:///us-east-1b/abc123"))
}

func newMockCloudProvider(nodeGroups []string, service *test.MockAutoscalingService, ec2Service *test.MockEc2Service) (*CloudProvider, error) {
	var err error

	cloudProvider := &CloudProvider{
		service:    service,
		ec2Service: ec2Service,
		nodeGroups: make(map[string]*NodeGroup, len(nodeGroups)),
	}

	configs := make([]cloudprovider.NodeGroupConfig, 0, len(nodeGroups))
	for _, i := range nodeGroups {
		configs = append(configs, cloudprovider.NodeGroupConfig{GroupID: i})
	}

	if service != nil {
		err = cloudProvider.RegisterNodeGroups(configs...)
	}

	return cloudProvider, err
}
