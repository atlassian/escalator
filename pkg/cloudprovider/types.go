package cloudprovider

import "fmt"

// NodeNotInNodeGroup is a special error type
// this happens when a node is not inside a expected node group
type NodeNotInNodeGroup struct {
	NodeName   string
	ProviderID string
	NodeGroup  string
}

func (ne *NodeNotInNodeGroup) Error() string {
	return fmt.Sprintf("node %v, %v belongs in a different node group than %v", ne.NodeName, ne.ProviderID, ne.NodeGroup)
}
