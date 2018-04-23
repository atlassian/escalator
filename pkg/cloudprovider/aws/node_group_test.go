package aws

import (
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNodeGroup_ID(t *testing.T) {
	id := "1"
	nodeGroup := NewNodeGroup(id, &autoscaling.Group{}, &CloudProvider{})
	assert.Equal(t, id, nodeGroup.ID())
}
