package controller

import (
	"github.com/atlassian/escalator/pkg/k8s"
	"k8s.io/api/core/v1"
	v1lister "k8s.io/client-go/listers/core/v1"
)

// Customer represents a model a customer running on our cluster
type NodeGroup struct {
	Name       				string
	LabelKey    			string
	LabelValue 				string
	// DaemonSetPercentUsage 	int64
	// minoverhead				int64
	// minNodes
	// maxNodes
}

// CustomerLister is just a light wrapper around a pod lister and node lister
// Used for grouping a customer and their listers
type CustomerLister struct {
	// Pod lister
	Pods k8s.PodLister
	// Node lister
	Nodes k8s.NodeLister
}

// NewPodAffinityFilterFunc creates a new PodFilterFunc based on filtering by namespaces
func NewPodAffinityFilterFunc(labelKey, labelValue string) k8s.PodFilterFunc {
	return func(pod *v1.Pod) bool {

		for _, ownerrefence := range pod.ObjectMeta.OwnerReferences{
			if ownerrefence.Kind == "DaemonSet"{
				return false
			}
		}

		if pod.Spec.Affinity != nil{
			for _, term := range pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
				for _, expression := range term.MatchExpressions{
					if expression.Key == labelKey{
						for _, value := range expression.Values{
							if value == labelValue{
								return true
							}
						}
					}
				}
			}
		}

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

		for _, ownerrefence := range pod.ObjectMeta.OwnerReferences{
			if ownerrefence.Kind == "DaemonSet"{
				return false
			}
		}

		if len(pod.Spec.NodeSelector) == 0 && pod.Spec.Affinity == nil{
			return true
		}
		return false
	}
}

// NewCustomerLister creates a new group from the backing lister and customer filter
func NewCustomerLister(allPodsLister v1lister.PodLister, allNodesLister v1lister.NodeLister, nodeGroup *NodeGroup) *CustomerLister {
	return &CustomerLister{
		k8s.NewFilteredPodsLister(allPodsLister, NewPodAffinityFilterFunc(nodeGroup.LabelKey, nodeGroup.LabelValue)),
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
