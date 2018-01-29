package controller

import (
	"github.com/atlassian/escalator/pkg/k8s"
	v1lister "k8s.io/client-go/listers/core/v1"
)

// CustomerLister is just a light wrapper around a pod lister and node lister
// Used for grouping a customer and their listers
type CustomerLister struct {
	// Pod lister
	Pods k8s.PodLister
	// Node lister
	Nodes k8s.NodeLister
}

// NewCustomerLister creates a new group from the backing lister and customer filter
func NewCustomerLister(allPodsLister v1lister.PodLister, allNodesLister v1lister.NodeLister, customer *Customer) *CustomerLister {
	return &CustomerLister{
		k8s.NewFilteredPodsLister(allPodsLister, customer.Namespaces),
		k8s.NewFilteredNodesLister(allNodesLister, customer.NodeLabels),
	}
}
