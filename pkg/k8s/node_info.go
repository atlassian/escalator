package k8s

import (
	"sync"

	v1 "k8s.io/api/core/v1"
)

// NodeInfo provides an abstraction on top of node to pods mappings
// replaces scheduler cache.NodeInfo that was removed from public in an older version of kubernetes
// Maintains the same interface
// NodeInfo this thread safe
type NodeInfo struct {
	lock sync.RWMutex

	node *v1.Node
	pods []*v1.Pod
}

// NewNodeInfo creates a new empty NodeInfo struct
func NewNodeInfo() *NodeInfo {
	return &NodeInfo{}
}

// AddPod adds a pod to the list of pods for this node
func (i *NodeInfo) AddPod(pod *v1.Pod) {
	i.lock.Lock()
	defer i.lock.Unlock()

	i.pods = append(i.pods, pod)
}

// Pods returns the list of pods for this node
func (i *NodeInfo) Pods() []*v1.Pod {
	i.lock.RLock()
	defer i.lock.RUnlock()

	return i.pods
}

// SetNode sets the current node that the pods belong to
func (i *NodeInfo) SetNode(node *v1.Node) {
	i.lock.Lock()
	defer i.lock.Unlock()

	i.node = node
}

// Node returns the current node for these pods
func (i *NodeInfo) Node() *v1.Node {
	i.lock.RLock()
	defer i.lock.RUnlock()

	return i.node
}
