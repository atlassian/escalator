package aws

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuilder_assumeRoleEnabled(t *testing.T) {
	// Test with ARN specified
	builder := &Builder{
		Opts: Opts{
			AssumeRoleARN: "arn:aws:iam::111111111111:role/escalator",
		},
	}
	assert.True(t, builder.assumeRoleEnabled())

	// Test with empty ARN
	builder = &Builder{
		Opts: Opts{
			AssumeRoleARN: "",
		},
	}
	assert.False(t, builder.assumeRoleEnabled())

	// Test with no ARN
	builder = &Builder{}
	assert.False(t, builder.assumeRoleEnabled())
}
