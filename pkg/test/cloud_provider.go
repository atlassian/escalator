package test

import (
	"github.com/atlassian/escalator/pkg/cloudprovider"
	"k8s.io/api/core/v1"
)

const ProviderName = "test"

// cloudProvider implements the CloudProvider interface
type CloudProvider struct {
	nodeGroups map[string]*NodeGroup
}

func NewCloudProvider(nodeGroupSize int) *CloudProvider {
	nodeGroups := make(map[string]*NodeGroup, nodeGroupSize)
	return &CloudProvider{nodeGroups}
}

func (c *CloudProvider) Name() string {
	return ProviderName
}

func (c *CloudProvider) NodeGroups() []cloudprovider.NodeGroup {
	// put the nodegroup concrete type into the abstract type
	ngs := make([]cloudprovider.NodeGroup, 0, len(c.nodeGroups))
	for _, ng := range c.nodeGroups {
		ngs = append(ngs, ng)
	}
	return ngs
}

func (c *CloudProvider) GetNodeGroup(id string) (cloudprovider.NodeGroup, bool) {
	ng, ok := c.nodeGroups[id]
	return ng, ok
}

func (c *CloudProvider) RegisterNodeGroups(ids ...string) error {
	return nil
}

func (c *CloudProvider) Refresh() error {
	return nil
}

func (c *CloudProvider) RegisterNodeGroup(nodeGroup *NodeGroup) {
	c.nodeGroups[nodeGroup.id] = nodeGroup
}

type NodeGroup struct {
	id         string
	minSize    int64
	maxSize    int64
	targetSize int64
}

func NewNodeGroup(id string, minSize int64, maxSize int64, targetSize int64) *NodeGroup {
	return &NodeGroup{
		id,
		minSize,
		maxSize,
		targetSize,
	}
}

func (n *NodeGroup) String() string {
	return n.id
}

func (n *NodeGroup) ID() string {
	return n.id
}

func (n *NodeGroup) MinSize() int64 {
	return n.minSize
}

func (n *NodeGroup) MaxSize() int64 {
	return n.maxSize
}

func (n *NodeGroup) TargetSize() int64 {
	return n.targetSize
}

func (n *NodeGroup) Size() int64 {
	return n.targetSize
}

func (n *NodeGroup) IncreaseSize(delta int64) error {
	return n.setASGDesiredSize(n.targetSize + delta)
}

func (n *NodeGroup) DeleteNodes(nodes ...*v1.Node) error {
	for range nodes {
		// Here we would normally tell AWS to terminate the instance and also decrement the desired capacity
		// but we just decrement the internal ASG size to reflect the remote change
		n.setASGDesiredSize(n.targetSize - 1)
	}
	return nil
}

func (n *NodeGroup) Belongs(node *v1.Node) bool {
	return false
}

func (n *NodeGroup) DecreaseTargetSize(delta int64) error {
	return n.setASGDesiredSize(n.targetSize + delta)
}

func (n *NodeGroup) Nodes() []string {
	return nil
}

func (n *NodeGroup) setASGDesiredSize(newSize int64) error {
	// This is where we would tell AWS or GCP to change the scaling group desired size
	// but we just update the internal target size of the node group to reflect the remote change
	n.targetSize = newSize
	return nil
}


