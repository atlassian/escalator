package test

import (
	"time"

	"github.com/atlassian/escalator/pkg/cloudprovider"
	v1 "k8s.io/api/core/v1"
)

// ProviderName is the mock cloud provider name for these tests
const ProviderName = "test"

// CloudProvider implements the CloudProvider interface
type CloudProvider struct {
	nodeGroups map[string]*NodeGroup
}

// NewCloudProvider creates a new test CloudProvider
func NewCloudProvider(nodeGroupSize int) *CloudProvider {
	nodeGroups := make(map[string]*NodeGroup, nodeGroupSize)
	return &CloudProvider{nodeGroups}
}

// Name mock implementation for test.CloudProvider
func (c *CloudProvider) Name() string {
	return ProviderName
}

// NodeGroups mock implementation for test.CloudProvider
func (c *CloudProvider) NodeGroups() []cloudprovider.NodeGroup {
	// put the nodegroup concrete type into the abstract type
	ngs := make([]cloudprovider.NodeGroup, 0, len(c.nodeGroups))
	for _, ng := range c.nodeGroups {
		ngs = append(ngs, ng)
	}
	return ngs
}

// GetNodeGroup mock implementation for test.CloudProvider
func (c *CloudProvider) GetNodeGroup(id string) (cloudprovider.NodeGroup, bool) {
	ng, ok := c.nodeGroups[id]
	return ng, ok
}

// RegisterNodeGroups mock implementation for test.CloudProvider
func (c *CloudProvider) RegisterNodeGroups(groups ...cloudprovider.NodeGroupConfig) error {
	return nil
}

// Refresh mock implementation for test.CloudProvider
func (c *CloudProvider) Refresh() error {
	return nil
}

// RegisterNodeGroup mock implementation for test.CloudProvider
func (c *CloudProvider) RegisterNodeGroup(nodeGroup *NodeGroup) {
	c.nodeGroups[nodeGroup.id] = nodeGroup
}

// GetInstance mock implementation for test.CloudProvider
func (c *CloudProvider) GetInstance(node *v1.Node) (cloudprovider.Instance, error) {
	return Instance{}, nil
}

// Instance mock implementation
type Instance struct {
	id string
}

// InstantiationTime mock for test.Instance
func (i Instance) InstantiationTime() time.Time {
	return time.Now()
}

// ID mock mock for test.Instance
func (i Instance) ID() string {
	return i.id
}

// NodeGroup is a mock implementation of NodeGroup for testing
type NodeGroup struct {
	id         string
	name       string
	minSize    int64
	maxSize    int64
	actualSize int64
	targetSize int64
}

// NewNodeGroup creates a new mock NodeGroup
func NewNodeGroup(id string, name string, minSize int64, maxSize int64, targetSize int64) *NodeGroup {
	return &NodeGroup{
		id,
		name,
		minSize,
		maxSize,
		targetSize,
		targetSize,
	}
}

// String mock implementation for NodeGroup
func (n *NodeGroup) String() string {
	return n.id
}

// ID mock implementation for NodeGroup
func (n *NodeGroup) ID() string {
	return n.id
}

// Name mock implementation for NodeGroup
func (n *NodeGroup) Name() string {
	return n.name
}

// MinSize mock implementation for NodeGroup
func (n *NodeGroup) MinSize() int64 {
	return n.minSize
}

// MaxSize mock implementation for NodeGroup
func (n *NodeGroup) MaxSize() int64 {
	return n.maxSize
}

// TargetSize mock implementation for NodeGroup
func (n *NodeGroup) TargetSize() int64 {
	return n.targetSize
}

// Size mock implementation for NodeGroup
func (n *NodeGroup) Size() int64 {
	return n.actualSize
}

// IncreaseSize mock implementation for NodeGroup
func (n *NodeGroup) IncreaseSize(delta int64) error {
	return n.setDesiredSize(n.targetSize + delta)
}

// DeleteNodes mock implementation for NodeGroup
func (n *NodeGroup) DeleteNodes(nodes ...*v1.Node) error {
	for range nodes {
		// Here we would normally tell the actual provider (AWS etc.) to terminate the instance and also decrement the
		// desired capacity, but we just decrement the internal size to reflect the remote change
		if err := n.setDesiredSize(n.targetSize - 1); err != nil {
			return err
		}
	}
	return nil
}

// Belongs mock implementation for NodeGroup
func (n *NodeGroup) Belongs(node *v1.Node) bool {
	return false
}

// DecreaseTargetSize mock implementation for NodeGroup
func (n *NodeGroup) DecreaseTargetSize(delta int64) error {
	return n.setDesiredSize(n.targetSize + delta)
}

// Nodes mock implementation for NodeGroup
func (n *NodeGroup) Nodes() []string {
	return nil
}

// setDesiredSize mock implementation for NodeGroup
func (n *NodeGroup) setDesiredSize(newSize int64) error {
	// This is where we would tell the actual provider (AWS etc.) to change the scaling group desired size
	// but we just update the internal target size of the node group to reflect the remote change
	n.targetSize = newSize
	n.actualSize = newSize
	return nil
}
