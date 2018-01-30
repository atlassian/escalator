package controller

import (
	"io"

	"github.com/atlassian/escalator/pkg/k8s"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	v1lister "k8s.io/client-go/listers/core/v1"
)

// DefaultCustomer is used for any pods that don't have a node selector defined
const DefaultCustomer = "default"

// NodeGroup represents a customer running on our cluster
// We differentiate customers by their node label
type NodeGroup struct {
	Name       string `json:"name" yaml:"name"`
	LabelKey   string `json:"label_key" yaml:"label_key"`
	LabelValue string `json:"label_value" yaml:"label_value"`

	DaemonSetPercentUsage int64 `json:"daemon_set_percent_usage,omitempty" yaml:"daemon_set_percent_usage,omitempty"`
	MinSlackSpace         int64 `json:"min_slack_space,omitempty" yaml:"min_slack_space,omitempty"`

	MinNodes int `json:"min_nodes,omitempty" yaml:"min_nodes,omitempty"`
	MaxNodes int `json:"max_nodes,omitempty" yaml:"max_nodes,omitempty"`
}

// UnmarshalNodeGroupsConfig decodes the yaml or json reader into a struct
func UnmarshalNodeGroupsConfig(reader io.Reader) ([]*NodeGroup, error) {
	var wrapper struct {
		Customers []*NodeGroup `json:"customers" yaml:"customers"`
	}
	if err := yaml.NewYAMLOrJSONDecoder(reader, 4096).Decode(&wrapper); err != nil {
		return []*NodeGroup{}, err
	}
	return wrapper.Customers, nil
}

// NodeGroupLister is just a light wrapper around a pod lister and node lister
// Used for grouping a customer and their listers
type NodeGroupLister struct {
	// Pod lister
	Pods k8s.PodLister
	// Node lister
	Nodes k8s.NodeLister
}

// NewPodAffinityFilterFunc creates a new PodFilterFunc based on filtering by label selectors
func NewPodAffinityFilterFunc(labelKey, labelValue string) k8s.PodFilterFunc {
	return func(pod *v1.Pod) bool {
		// Filter out DaemonSets in our calcuation
		for _, ownerrefence := range pod.ObjectMeta.OwnerReferences {
			if ownerrefence.Kind == "DaemonSet" {
				return false
			}
		}

		// check the node selector
		if value, ok := pod.Spec.NodeSelector[labelKey]; ok {
			if value == labelValue {
				return true
			}
		}

		// finally, if the pod has an affinity for our selector then we will include it
		if pod.Spec.Affinity != nil && pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
			for _, term := range pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
				for _, expression := range term.MatchExpressions {
					if expression.Key == labelKey {
						for _, value := range expression.Values {
							if value == labelValue {
								return true
							}
						}
					}
				}
			}
		}

		return false
	}
}

// NewPodDefaultFilterFunc creates a new PodFilterFunc that includes pods that do not have a selector
func NewPodDefaultFilterFunc() k8s.PodFilterFunc {
	return func(pod *v1.Pod) bool {
		// filter out daemonsets
		for _, ownerReference := range pod.ObjectMeta.OwnerReferences {
			if ownerReference.Kind == "DaemonSet" {
				return false
			}
		}

		// allow pods without a node selector and without a pod affinity
		return len(pod.Spec.NodeSelector) == 0 && pod.Spec.Affinity == nil
	}
}

// NewNodeLabelFilterFunc creates a new NodeFilterFunc based on filtering by node labels
func NewNodeLabelFilterFunc(labelKey, labelValue string) k8s.NodeFilterFunc {
	return func(node *v1.Node) bool {
		if value, ok := node.ObjectMeta.Labels[labelKey]; ok {
			if value == labelValue {
				return true
			}
		}
		return false
	}
}

// NewNodeGroupLister creates a new group from the backing lister and customer filter
func NewNodeGroupLister(allPodsLister v1lister.PodLister, allNodesLister v1lister.NodeLister, nodeGroup *NodeGroup) *NodeGroupLister {
	return &NodeGroupLister{
		k8s.NewFilteredPodsLister(allPodsLister, NewPodAffinityFilterFunc(nodeGroup.LabelKey, nodeGroup.LabelValue)),
		k8s.NewFilteredNodesLister(allNodesLister, NewNodeLabelFilterFunc(nodeGroup.LabelKey, nodeGroup.LabelValue)),
	}
}

// NewDefaultNodeGroupLister creates a new group from the backing lister and customer filter with the default filter
func NewDefaultNodeGroupLister(allPodsLister v1lister.PodLister, allNodesLister v1lister.NodeLister, nodeGroup *NodeGroup) *NodeGroupLister {
	return &NodeGroupLister{
		k8s.NewFilteredPodsLister(allPodsLister, NewPodDefaultFilterFunc()),
		k8s.NewFilteredNodesLister(allNodesLister, NewNodeLabelFilterFunc(nodeGroup.LabelKey, nodeGroup.LabelValue)),
	}
}
