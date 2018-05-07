package aws

import (
	"fmt"
	"github.com/atlassian/escalator/pkg/cloudprovider"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	log "github.com/sirupsen/logrus"
	"time"
)

// Builder builds the aws cloud provider
type Builder struct {
	ProviderOpts cloudprovider.BuildOpts
	Opts         Opts
}

type Opts struct {
	AssumeRoleARN string
}

const AssumeRoleNamePrefix = "atlassian-escalator"

// Build the cloud provider
func (b Builder) Build() (cloudprovider.CloudProvider, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}

	var creds *credentials.Credentials

	// If assume role is enabled, create credentials with the ARN
	if b.assumeRoleEnabled() {
		creds = stscreds.NewCredentials(sess, b.Opts.AssumeRoleARN, setAssumeRoleName)
	}

	// Create the autoscaling service
	service := autoscaling.New(sess, &aws.Config{
		Credentials: creds,
	})
	cloud := &CloudProvider{
		service:    service,
		nodeGroups: make(map[string]*NodeGroup, len(b.ProviderOpts.NodeGroupIDs)),
	}

	// Register the node groups
	err = cloud.RegisterNodeGroups(b.ProviderOpts.NodeGroupIDs...)
	if err != nil {
		return nil, err
	}

	// Log the provider we used
	credValue, err := service.Client.Config.Credentials.Get()
	if err != nil {
		return nil, err
	}
	log.Infof("aws session created successfully, using provider %v", credValue.ProviderName)

	return cloud, nil
}

// assumeRoleEnabled returns whether assume role is enabled
func (b Builder) assumeRoleEnabled() bool {
	return len(b.Opts.AssumeRoleARN) > 0
}

// setAssumeRoleName allows setting of a custom RoleSessionName for assume role
func setAssumeRoleName(provider *stscreds.AssumeRoleProvider) {
	provider.RoleSessionName = fmt.Sprintf("%v-%d", AssumeRoleNamePrefix, time.Now().UTC().UnixNano())
}
