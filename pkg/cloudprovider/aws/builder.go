package aws

import (
	"github.com/atlassian/escalator/pkg/cloudprovider"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	log "github.com/sirupsen/logrus"
)

// Builder builds the aws cloudprovider
type Builder struct {
	ProviderOpts cloudprovider.BuildOpts
}

// Build the cloudprovider
func (b Builder) Build() cloudprovider.CloudProvider {
	sess, err := session.NewSession()
	if err != nil {
		log.Fatalln("Failed to create aws session")
		return nil
	}
	service := autoscaling.New(sess)
	if service == nil {
		log.Fatalln("Failed to create aws autoscaling service")
		return nil
	}

	cloud := &CloudProvider{
		service:    service,
		nodeGroups: make(map[string]*NodeGroup, len(b.ProviderOpts.NodeGroupIDs)),
	}

	err = cloud.RegisterNodeGroups(b.ProviderOpts.NodeGroupIDs...)
	if err != nil {
		log.Fatalln(err)
		return nil
	}

	log.Infoln("aws session created successfully")

	return cloud
}
