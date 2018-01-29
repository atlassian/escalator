package controller

import (
	"github.com/atlassian/escalator/pkg/k8s"
	"k8s.io/api/core/v1"
	v1lister "k8s.io/client-go/listers/core/v1"
)

// Customer represents a model a customer running on our cluster
type Customer struct {
	Name       string
	Namespaces []string
	NodeLabels []string
}

// CustomerLister is just a light wrapper around a pod lister and node lister
// Used for grouping a customer and their listers
type CustomerLister struct {
	// Pod lister
	Pods k8s.PodLister
	// Node lister
	Nodes k8s.NodeLister
}

// NewPodNamespaceFilterFunc creates a new PodFilterFunc based on filtering by namespaces
func NewPodNamespaceFilterFunc(namespaces []string) k8s.PodFilterFunc {
	return func(pod *v1.Pod) bool {
		for _, namespace := range namespaces {
			if pod.Namespace == namespace {
				return true
			}
		}
		return false
	}
}

// NewNodeLabelFilterFunc creates a new NodeFilterFunc based on filtering by node labels
func NewNodeLabelFilterFunc(namespaces []string) k8s.NodeFilterFunc {
	return func(pod *v1.Node) bool {
		// TODO: filter by labels
		return true
	}
}

// NewCustomerLister creates a new group from the backing lister and customer filter
func NewCustomerLister(allPodsLister v1lister.PodLister, allNodesLister v1lister.NodeLister, customer *Customer) *CustomerLister {
	return &CustomerLister{
		k8s.NewFilteredPodsLister(allPodsLister, NewPodNamespaceFilterFunc(customer.Namespaces)),
		k8s.NewFilteredNodesLister(allNodesLister, NewNodeLabelFilterFunc(customer.NodeLabels)),
	}
}
