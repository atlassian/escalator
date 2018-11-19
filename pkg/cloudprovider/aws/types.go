package aws

import "fmt"

// AssumeRoleNamePrefix is the assume role session name prefix
const AssumeRoleNamePrefix = "atlassian-escalator"

// Opts includes options for AWS cloud provider
type Opts struct {
	AssumeRoleARN string
}

// NodeNotInAutoScalingGroup is a special error type
// this happens when a node is not inside a expected node group
type NodeNotInAutoScalingGroup struct {
	NodeName   string
	ProviderID string
	NodeGroup  string
}

func (ne *NodeNotInAutoScalingGroup) Error() string {
	return fmt.Sprintf("node %v, %v belongs in a different asg than %v", ne.NodeName, ne.ProviderID, ne.NodeGroup)
}
