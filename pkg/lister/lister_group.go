package lister

import v1lister "k8s.io/client-go/listers/core/v1"

// Group is just a light wrapper around a few listers
type Group struct {
	// Pod lister
	Pods PodLister
	// Node lister
	Nodes NodeLister
}

func NewGroup(allPodsLister v1lister.PodLister, allNodesLister v1lister.NodeLister, namespaces, nodeLabels []string) *Group {
	return &Group{
		NewFilteredPodsLister(allPodsLister, namespaces),
		NewFilteredNodesLister(allNodesLister, nodeLabels),
	}
}
