package controller

import (
	"github.com/atlassian/escalator/pkg/k8s"
	"k8s.io/api/core/v1"
	v1lister "k8s.io/client-go/listers/core/v1"
)

// Customer represents a model a customer running on our cluster
type NodeGroup struct {
	Name       string
	LabelKey    string
	LabelValue string
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
func NewPodNamespaceFilterFunc(labelKey, labelValue string) k8s.PodFilterFunc {
	return func(pod *v1.Pod) bool {
		if value, ok := pod.Spec.NodeSelector[labelKey]; ok{
			if value == labelValue{
				return true
			}
		}
		return false
	}
}

// NewNodeLabelFilterFunc creates a new NodeFilterFunc based on filtering by node labels
func NewNodeLabelFilterFunc(labelKey, labelValue string) k8s.NodeFilterFunc {
	return func(node *v1.Node) bool {
		if value, ok := node.ObjectMeta.Labels[labelKey]; ok{
			if value == labelValue{
				return true
			}
		}
		return false
	}
}

func NewDefaultPodLister() k8s.PodFilterFunc{
	return func(pod *v1.Pod) bool {
		if len(pod.Spec.NodeSelector) == 0{
			return true
		}
		return false
	}
}

// NewCustomerLister creates a new group from the backing lister and customer filter
func NewCustomerLister(allPodsLister v1lister.PodLister, allNodesLister v1lister.NodeLister, nodeGroup *NodeGroup) *CustomerLister {
	return &CustomerLister{
		k8s.NewFilteredPodsLister(allPodsLister, NewPodNamespaceFilterFunc(nodeGroup.LabelKey, nodeGroup.LabelValue)),
		k8s.NewFilteredNodesLister(allNodesLister, NewNodeLabelFilterFunc(nodeGroup.LabelKey, nodeGroup.LabelValue)),
	}
}

// NewDefaultLister creates a new group from the backing lister and customer filter
func NewDefaultLister(allPodsLister v1lister.PodLister, allNodesLister v1lister.NodeLister, nodeGroup *NodeGroup) *CustomerLister {
	return &CustomerLister{
		k8s.NewFilteredPodsLister(allPodsLister, NewDefaultPodLister()),
		k8s.NewFilteredNodesLister(allNodesLister, NewNodeLabelFilterFunc(nodeGroup.LabelKey, nodeGroup.LabelValue)),
	}
}
