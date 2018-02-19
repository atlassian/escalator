package aws

import (
	"fmt"
	"github.com/atlassian/escalator/pkg/cloudprovider"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"k8s.io/api/core/v1"
)

// ProviderName identifies this module as aws
const ProviderName = "aws"

// ErrorNotImplemented indicates that a method or function has not been implemented yet
var ErrorNotImplemented = fmt.Errorf("method not implemented")

// CloudProvider providers an aws cloudprovider implementation
type CloudProvider struct {
	service *autoscaling.AutoScaling
}

// Name returns name of the cloud provider.
func (c *CloudProvider) Name() string {
	return ProviderName
}

// NodeGroups returns all node groups configured for this cloud provider.
func (c *CloudProvider) NodeGroups() []cloudprovider.NodeGroup {
	return nil
}

// GetNodeGroup gets the node group from the coudprovider. Returns if it exists or not
func (c *CloudProvider) GetNodeGroup(id string) (cloudprovider.NodeGroup, bool) {
	return nil, false
}

// Refresh is called before every main loop and can be used to dynamically update cloud provider state.
func (c *CloudProvider) Refresh() error {
	return ErrorNotImplemented
}

// Cleanup cleans up open resources before the cloud provider is destroyed, i.e. go routines etc.
func (c *CloudProvider) Cleanup() error {
	return ErrorNotImplemented
}

// NodeGroup implements a aws nodegroup
type NodeGroup struct {
}

func (n *NodeGroup) String() string {
	return ""
}

// ID returns an unique identifier of the node group.
func (n *NodeGroup) ID() string {
	return ""
}

// MinSize returns minimum size of the node group.
func (n *NodeGroup) MinSize() int {
	return 0
}

// MaxSize returns maximum size of the node group.
func (n *NodeGroup) MaxSize() int {
	return 0
}

// TargetSize returns the current target size of the node group. It is possible that the
// number of nodes in Kubernetes is different at the moment but should be equal
// to Size() once everything stabilizes (new nodes finish startup and registration or
// removed nodes are deleted completely).
func (n *NodeGroup) TargetSize() (int, error) {
	return 0, ErrorNotImplemented
}

// IncreaseSize increases the size of the node group. To delete a node you need
// to explicitly name it and use DeleteNode. This function should wait until
// node group size is updated.
func (n *NodeGroup) IncreaseSize(delta int) error {
	return ErrorNotImplemented
}

// DeleteNodes deletes nodes from this node group. Error is returned either on
// failure or if the given node doesn't belong to this node group. This function
// should wait until node group size is updated.
func (n *NodeGroup) DeleteNodes(...*v1.Node) error {
	return ErrorNotImplemented
}

// DecreaseTargetSize decreases the target size of the node group. This function
// doesn't permit to delete any existing node and can be used only to reduce the
// request for new nodes that have not been yet fulfilled. Delta should be negative.
// It is assumed that cloud provider will not delete the existing nodes when there
// is an option to just decrease the target.
func (n *NodeGroup) DecreaseTargetSize(delta int) error {
	return ErrorNotImplemented
}

// Nodes returns a list of all nodes that belong to this node group.
func (n *NodeGroup) Nodes() ([]string, error) {
	return nil, ErrorNotImplemented
}
