package k8s

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	v1lister "k8s.io/client-go/listers/core/v1"
)

// NodeFilterFunc provides a definition for a predicate based on matching a node
// return true for keep node
type NodeFilterFunc func(*v1.Node) bool

// NodeLister provides an interface for anything that can list a node
type NodeLister interface {
	List() ([]*v1.Node, error)
}

// FilteredNodesLister lists nodes filtered by labels
type FilteredNodesLister struct {
	nodeLister v1lister.NodeLister
	filterFunc NodeFilterFunc
}

// NewFilteredNodesLister creates a new lister and informerSynced for all nodes filter by nodegroup (nodeLabels)
func NewFilteredNodesLister(nodeLister v1lister.NodeLister, filterFunc NodeFilterFunc) NodeLister {
	return &FilteredNodesLister{
		nodeLister,
		filterFunc,
	}
}

// List lists all nodes from the cache filtered by labels
func (lister *FilteredNodesLister) List() ([]*v1.Node, error) {
	var filteredNodes []*v1.Node
	allNodes, err := lister.nodeLister.List(labels.Everything())
	if err != nil {
		return filteredNodes, err
	}

	// only include node that match the filtering function
	for _, node := range allNodes {
		if lister.filterFunc(node) {
			filteredNodes = append(filteredNodes, node)
		}
	}

	return filteredNodes, nil
}
