package aws

import (
	"github.com/atlassian/escalator/pkg/cloudprovider"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	log "github.com/sirupsen/logrus"
)

// Builder builds the aws cloudprivider
type Builder struct {
	ProviderOpts cloudprovider.BuildOpts
}

// Build the cloudprovider
func (b Builder) Build() cloudprovider.CloudProvider {
	service := autoscaling.New(session.New())
	if service == nil {
		log.Fatalln("Failed to create aws autoscaling service")
		return nil
	}
	//service.Config.Region = aws.String("")

	cloud := &CloudProvider{
		service: service,
	}

	return cloud
}
