package aws

// AssumeRoleNamePrefix is the assume role session name prefix
const AssumeRoleNamePrefix = "atlassian-escalator"

// Opts includes options for AWS cloud provider
type Opts struct {
	AssumeRoleARN string
}
