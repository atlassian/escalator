package aws

import (
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
