package k8s

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	v1lister "k8s.io/client-go/listers/core/v1"
)

// NodeLister provides an interface for anything that can list a node
type NodeLister interface {
	List() ([]*v1.Node, error)
}

// FilteredNodesLister lists nodes filtered by labels
type FilteredNodesLister struct {
	nodeLister v1lister.NodeLister
	nodeLabels []string
}

// NewFilteredNodesLister creates a new lister and informerSynced for all nodes filter by customer (nodeLabels)
func NewFilteredNodesLister(nodeLister v1lister.NodeLister, nodeLabels []string) NodeLister {
	return &FilteredNodesLister{
		nodeLister,
		nodeLabels,
	}
}

// List lists all nodes from the cache filtered by labels
func (lister *FilteredNodesLister) List() ([]*v1.Node, error) {
	// TODO: filter by labels
	return lister.nodeLister.List(labels.Everything())
}
