package aws

import (
	"github.com/atlassian/escalator/pkg/cloudprovider"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	log "github.com/sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
)

// Builder builds the aws cloudprovider
type Builder struct {
	ProviderOpts cloudprovider.BuildOpts
	AssumeRoleARN   string
}

// Build the cloudprovider
func (b Builder) Build() (cloudprovider.CloudProvider, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}

	var creds *credentials.Credentials
	if b.assumeRoleEnabled() {
		creds = stscreds.NewCredentials(sess, b.AssumeRoleARN)
	}

	service := autoscaling.New(sess, &aws.Config{
		Credentials: creds,
	})
	cloud := &CloudProvider{
		service:    service,
		nodeGroups: make(map[string]*NodeGroup, len(b.ProviderOpts.NodeGroupIDs)),
	}

	err = cloud.RegisterNodeGroups(b.ProviderOpts.NodeGroupIDs...)
	if err != nil {
		return nil, err
	}

	credValue, err := service.Client.Config.Credentials.Get()
	if err != nil {
		return nil, err
	}

	log.Infof("aws session created successfully, using provider %v", credValue.ProviderName)
	return cloud, nil
}

// assumeRoleEnabled returns whether assume role is enabled
func (b Builder) assumeRoleEnabled() bool {
	return len(b.AssumeRoleARN) > 0
}
