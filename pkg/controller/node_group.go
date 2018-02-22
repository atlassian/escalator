package controller

import (
	"fmt"
	"io"
	"time"

	"github.com/atlassian/escalator/pkg/k8s"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	v1lister "k8s.io/client-go/listers/core/v1"
)

// DefaultNodeGroup is used for any pods that don't have a node selector defined
const DefaultNodeGroup = "default"

// NodeGroupOptions represents a nodegroup running on our cluster
// We differentiate nodegroups by their node label
type NodeGroupOptions struct {
	Name             string `json:"name,omitempty" yaml:"name,omitempty"`
	LabelKey         string `json:"label_key,omitempty" yaml:"label_key,omitempty"`
	LabelValue       string `json:"label_value,omitempty" yaml:"label_value,omitempty"`
	CloudProviderASG string `json:"cloud_provider_asg,omitempty" yaml:"cloud_provider_asg,omitempty"`

	MinNodes int `json:"min_nodes,omitempty" yaml:"min_nodes,omitempty"`
	MaxNodes int `json:"max_nodes,omitempty" yaml:"max_nodes,omitempty"`

	DryMode bool `json:"dry_mode,omitempty" yaml:"dry_mode,omitempty"`

	TaintUpperCapacityThreshholdPercent int `json:"taint_upper_capacity_threshhold_percent,omitempty" yaml:"taint_upper_capacity_threshhold_percent,omitempty"`
	TaintLowerCapacityThreshholdPercent int `json:"taint_lower_capacity_threshhold_percent,omitempty" yaml:"taint_lower_capacity_threshhold_percent,omitempty"`

	ScaleUpThreshholdPercent int `json:"scale_up_threshhold_percent,omitempty" yaml:"scale_up_threshhold_percent,omitempty"`

	SlowNodeRemovalRate int `json:"slow_node_removal_rate,omitempty" yaml:"slow_node_removal_rate,omitempty"`
	FastNodeRemovalRate int `json:"fast_node_removal_rate,omitempty" yaml:"fast_node_removal_rate,omitempty"`

	SoftDeleteGracePeriod string `json:"soft_delete_grace_period,omitempty" yaml:"soft_delete_grace_period,omitempty"`
	HardDeleteGracePeriod string `json:"hard_delete_grace_period,omitempty" yaml:"soft_delete_grace_period,omitempty"`

	ScaleUpCoolDownPeriod string `json:"scale_up_cool_down_period,omitempty" yaml:"scale_up_cool_down_period,omitempty"`

	// Private variables for storing the parsed duration from the string
	softDeleteGracePeriodDuration time.Duration
	hardDeleteGracePeriodDuration time.Duration
	scaleUpCoolDownPeriodDuration time.Duration

	// DEPRECATED
	UntaintUpperCapacityThreshholdPercent int `json:"untaint_upper_capacity_threshhold_percent,omitempty" yaml:"untaint_upper_capacity_threshhold_percent,omitempty"`
	UntaintLowerCapacityThreshholdPercent int `json:"untaint_lower_capacity_threshhold_percent,omitempty" yaml:"untaint_lower_capacity_threshhold_percent,omitempty"`
	SlowNodeRevivalRate                   int `json:"slow_node_revival_rate,omitempty" yaml:"slow_node_revival_rate,omitempty"`
	FastNodeRevivalRate                   int `json:"fast_node_revival_rate,omitempty" yaml:"fast_node_revival_rate,omitempty"`

	// UNUSED
	SoftTaintEffectPercent         int     `json:"soft_taint_effect_percent,omitempty" yaml:"soft_taint_effect_percent,omitempty"`
	DampeningStrength              float64 `json:"dampening_strength,omitempty" yaml:"dampening_strength,omitempty"`
	DaemonSetUsagePercent          int     `json:"daemon_set_usage_percent,omitempty" yaml:"daemon_set_usage_percent,omitempty"`
	MinSlackSpacePercent           int     `json:"min_slack_space_percent,omitempty" yaml:"min_slack_space_percent,omitempty"`
	ScaleDownMinGracePeriodSeconds int     `json:"scale_down_min_grace_period_seconds,omitempty" yaml:"scale_down_min_grace_period_seconds,omitempty"`
}

// UnmarshalNodeGroupOptions decodes the yaml or json reader into a struct
func UnmarshalNodeGroupOptions(reader io.Reader) ([]NodeGroupOptions, error) {
	var wrapper struct {
		NodeGroups []NodeGroupOptions `json:"node_groups" yaml:"node_groups"`
	}
	if err := yaml.NewYAMLOrJSONDecoder(reader, 4096).Decode(&wrapper); err != nil {
		return []NodeGroupOptions{}, err
	}
	return wrapper.NodeGroups, nil
}

// ValidateNodeGroup is a safety check to validate that a nodegroup has valid options
func ValidateNodeGroup(nodegroup NodeGroupOptions) []error {
	var problems []error

	checkThat := func(cond bool, format string, output ...interface{}) {
		if !cond {
			problems = append(problems, fmt.Errorf(format, output...))
		}
	}

	checkThat(len(nodegroup.Name) > 0, "name cannot be empty")
	checkThat(len(nodegroup.LabelKey) > 0, "labelkey cannot be empty")
	checkThat(len(nodegroup.LabelValue) > 0, "labelvalue cannot be empty")
	checkThat(len(nodegroup.CloudProviderASG) > 0, "cloudprovider nodegroup asg cannot be empty")

	checkThat(nodegroup.TaintUpperCapacityThreshholdPercent >= 0, "taint upper capacity must be larger than 0")
	checkThat(nodegroup.TaintLowerCapacityThreshholdPercent >= 0, "taint lower capacity must be larger than 0")
	checkThat(nodegroup.ScaleUpThreshholdPercent >= 0, "scale up threshhold should be larger than 0")

	checkThat(nodegroup.TaintLowerCapacityThreshholdPercent < nodegroup.TaintUpperCapacityThreshholdPercent,
		"lower taint threshhold must be lower than upper taint threshold")
	checkThat(nodegroup.TaintUpperCapacityThreshholdPercent < nodegroup.ScaleUpThreshholdPercent,
		"taint upper capacity threshold should be lower than scale up threshold")

	checkThat(nodegroup.MinNodes < nodegroup.MaxNodes, "min nodes must be smaller than max nodes")
	checkThat(nodegroup.MaxNodes >= 0, "max nodes must be larger than 0")
	checkThat(nodegroup.SlowNodeRemovalRate <= nodegroup.FastNodeRemovalRate, "slow removal rate must be smaller than fast removal rate")

	checkThat(nodegroup.UntaintLowerCapacityThreshholdPercent == 0, "UntaintLowerCapacityThreshholdPercent is deprecated. please use ScaleUpThreshholdPercent")
	checkThat(nodegroup.UntaintUpperCapacityThreshholdPercent == 0, "UntaintUpperCapacityThreshholdPercent is deprecated. please use ScaleUpThreshholdPercent")

	checkThat(len(nodegroup.SoftDeleteGracePeriod) > 0, "soft grace period must not be empty")
	checkThat(len(nodegroup.HardDeleteGracePeriod) > 0, "hard grace period must not be empty")
	checkThat(len(nodegroup.ScaleUpCoolDownPeriod) > 0, "scale up cooldown period must not be empty")

	checkThat(nodegroup.SoftDeleteGracePeriodDuration() > 0, "soft grace period failed to parse into a time.Duration. check your formatting.")
	checkThat(nodegroup.HardDeleteGracePeriodDuration() > 0, "hard grace period failed to parse into a time.Duration. check your formatting.")
	checkThat(nodegroup.SoftDeleteGracePeriodDuration() < nodegroup.HardDeleteGracePeriodDuration(), "soft grace period must be less than hard grace period")

	checkThat(nodegroup.ScaleUpCoolDownPeriodDuration() >= 0, "scale up cooldown period duration must be positive")

	return problems
}

// SoftDeleteGracePeriodDuration lazily returns/parses the softDeleteGracePeriod string into a duration
func (n *NodeGroupOptions) SoftDeleteGracePeriodDuration() time.Duration {
	if n.softDeleteGracePeriodDuration == 0 {
		duration, err := time.ParseDuration(n.SoftDeleteGracePeriod)
		if err != nil {
			return 0
		}
		n.softDeleteGracePeriodDuration = duration
	}

	return n.softDeleteGracePeriodDuration
}

// HardDeleteGracePeriodDuration lazily returns/parses the hardDeleteGracePeriodDuration string into a duration
func (n *NodeGroupOptions) HardDeleteGracePeriodDuration() time.Duration {
	if n.hardDeleteGracePeriodDuration == 0 {
		duration, err := time.ParseDuration(n.HardDeleteGracePeriod)
		if err != nil {
			return 0
		}
		n.hardDeleteGracePeriodDuration = duration
	}

	return n.hardDeleteGracePeriodDuration
}

// ScaleUpCoolDownPeriodDuration lazily returns/parses the scaleUpCoolDownPeriod string into a duration
func (n *NodeGroupOptions) ScaleUpCoolDownPeriodDuration() time.Duration {
	if n.scaleUpCoolDownPeriodDuration == 0 {
		duration, err := time.ParseDuration(n.ScaleUpCoolDownPeriod)
		if err != nil {
			return 0
		}
		n.scaleUpCoolDownPeriodDuration = duration
	}

	return n.scaleUpCoolDownPeriodDuration
}

// NodeGroupLister is just a light wrapper around a pod lister and node lister
// Used for grouping a nodegroup and their listers
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
		if k8s.PodIsDaemonSet(pod) {
			return false
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
		if node.Spec.Unschedulable {
			return false
		}

		if value, ok := node.ObjectMeta.Labels[labelKey]; ok {
			if value == labelValue {
				return true
			}
		}
		return false
	}
}

// NewNodeGroupLister creates a new group from the backing lister and nodegroup filter
func NewNodeGroupLister(allPodsLister v1lister.PodLister, allNodesLister v1lister.NodeLister, nodeGroup NodeGroupOptions) *NodeGroupLister {
	return &NodeGroupLister{
		k8s.NewFilteredPodsLister(allPodsLister, NewPodAffinityFilterFunc(nodeGroup.LabelKey, nodeGroup.LabelValue)),
		k8s.NewFilteredNodesLister(allNodesLister, NewNodeLabelFilterFunc(nodeGroup.LabelKey, nodeGroup.LabelValue)),
	}
}

// NewDefaultNodeGroupLister creates a new group from the backing lister and nodegroup filter with the default filter
func NewDefaultNodeGroupLister(allPodsLister v1lister.PodLister, allNodesLister v1lister.NodeLister, nodeGroup NodeGroupOptions) *NodeGroupLister {
	return &NodeGroupLister{
		k8s.NewFilteredPodsLister(allPodsLister, NewPodDefaultFilterFunc()),
		k8s.NewFilteredNodesLister(allNodesLister, NewNodeLabelFilterFunc(nodeGroup.LabelKey, nodeGroup.LabelValue)),
	}
}
