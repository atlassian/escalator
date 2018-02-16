package aws

import (
	"github.com/atlassian/escalator/pkg/cloudprovider"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
)

// ProviderName identifies this module as aws
const ProviderName = "aws"

func importStub() {
	service := autoscaling.New(session.New())
	_ = service
}

// Builder builds the aws cloudprivider
type Builder struct {
	ProviderOpts cloudprovider.BuildOpts
}

// Build the cloudprovider
func (b Builder) Build() cloudprovider.CloudProvider {
	// TODO(jgonzalez): Not implemented
	return nil
}
