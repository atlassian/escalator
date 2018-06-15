package controller

import (
	"fmt"
	"io"
	"time"

	"errors"
	"github.com/atlassian/escalator/pkg/cloudprovider"
	"github.com/atlassian/escalator/pkg/k8s"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	v1lister "k8s.io/client-go/listers/core/v1"
)

// DefaultNodeGroup is used for any pods that don't have a node selector defined
const DefaultNodeGroup = "default"

// TaintSelectionMethods are the allowed methods for selecting which nodes to taint
// oldest = selects the oldest nodes in the node group and taints them first
// drainable = selects the nodes that are easily drainable and taints them first. The cluster-autoscaler drain simulator
// is used to determine if the node is easily drainable.
var TaintSelectionMethods = []string{"oldest", "drainable"}

// NodeGroupOptions represents a nodegroup running on our cluster
// We differentiate nodegroups by their node label
type NodeGroupOptions struct {
	Name                   string `json:"name,omitempty" yaml:"name,omitempty"`
	LabelKey               string `json:"label_key,omitempty" yaml:"label_key,omitempty"`
	LabelValue             string `json:"label_value,omitempty" yaml:"label_value,omitempty"`
	CloudProviderGroupName string `json:"cloud_provider_group_name,omitempty" yaml:"cloud_provider_group_name,omitempty"`

	MinNodes int `json:"min_nodes,omitempty" yaml:"min_nodes,omitempty"`
	MaxNodes int `json:"max_nodes,omitempty" yaml:"max_nodes,omitempty"`

	DryMode bool `json:"dry_mode,omitempty" yaml:"dry_mode,omitempty"`

	TaintUpperCapacityThresholdPercent int `json:"taint_upper_capacity_threshold_percent,omitempty" yaml:"taint_upper_capacity_threshold_percent,omitempty"`
	TaintLowerCapacityThresholdPercent int `json:"taint_lower_capacity_threshold_percent,omitempty" yaml:"taint_lower_capacity_threshold_percent,omitempty"`

	ScaleUpThresholdPercent int `json:"scale_up_threshold_percent,omitempty" yaml:"scale_up_threshold_percent,omitempty"`

	SlowNodeRemovalRate int `json:"slow_node_removal_rate,omitempty" yaml:"slow_node_removal_rate,omitempty"`
	FastNodeRemovalRate int `json:"fast_node_removal_rate,omitempty" yaml:"fast_node_removal_rate,omitempty"`

	SoftDeleteGracePeriod string `json:"soft_delete_grace_period,omitempty" yaml:"soft_delete_grace_period,omitempty"`
	HardDeleteGracePeriod string `json:"hard_delete_grace_period,omitempty" yaml:"soft_delete_grace_period,omitempty"`

	ScaleUpCoolDownPeriod string `json:"scale_up_cool_down_period,omitempty" yaml:"scale_up_cool_down_period,omitempty"`

	DrainBeforeTermination bool               `json:"drain_before_termination,omitempty" yaml:"drain_before_termination,omitempty"`
	TaintSelectionMethods  map[string]float64 `json:"taint_selection_methods,omitempty" yaml:"taint_selection_methods,omitempty"`

	// Private variables for storing the parsed duration from the string
	softDeleteGracePeriodDuration time.Duration
	hardDeleteGracePeriodDuration time.Duration
	scaleUpCoolDownPeriodDuration time.Duration
}

// UnmarshalNodeGroupOptions decodes the yaml or json reader into a struct
func UnmarshalNodeGroupOptions(reader io.Reader) ([]NodeGroupOptions, error) {
	var wrapper struct {
		NodeGroups []NodeGroupOptions `json:"node_groups" yaml:"node_groups"`
	}
	if err := yaml.NewYAMLOrJSONDecoder(reader, 4096).Decode(&wrapper); err != nil {
		return []NodeGroupOptions{}, err
	}
	for i, option := range wrapper.NodeGroups {
		wrapper.NodeGroups[i] = NodeGroupOptionsDefaults(option)
	}
	return wrapper.NodeGroups, nil
}

// NodeGroupOptionsDefaults sets the default values if they haven't been set in the config
func NodeGroupOptionsDefaults(options NodeGroupOptions) NodeGroupOptions {
	// Set the default TaintSelectionMethod
	if len(options.TaintSelectionMethods) == 0 {
		options.TaintSelectionMethods = map[string]float64{
			"oldest": float64(1),
		}
	}
	return options
}

// ValidateNodeGroup is a safety check to validate that a nodegroup has valid options
func ValidateNodeGroup(nodegroup NodeGroupOptions) (problems []error) {
	checkThat := func(cond bool, format string, output ...interface{}) {
		if !cond {
			problems = append(problems, fmt.Errorf(format, output...))
		}
	}

	checkThat(len(nodegroup.Name) > 0, "name cannot be empty")
	checkThat(len(nodegroup.LabelKey) > 0, "label_key cannot be empty")
	checkThat(len(nodegroup.LabelValue) > 0, "label_value cannot be empty")
	checkThat(len(nodegroup.CloudProviderGroupName) > 0, "cloud_provider_group_name cannot be empty")

	checkThat(nodegroup.TaintUpperCapacityThresholdPercent > 0, "taint_upper_capacity_threshold_percent must be larger than 0")
	checkThat(nodegroup.TaintLowerCapacityThresholdPercent > 0, "taint_lower_capacity_threshold_percent must be larger than 0")
	checkThat(nodegroup.ScaleUpThresholdPercent > 0, "scale_up_threshold_percent must be larger than 0")

	checkThat(nodegroup.TaintLowerCapacityThresholdPercent < nodegroup.TaintUpperCapacityThresholdPercent,
		"taint_lower_capacity_threshold_percent must be less than taint_upper_capacity_threshold_percent")
	checkThat(nodegroup.TaintUpperCapacityThresholdPercent < nodegroup.ScaleUpThresholdPercent,
		"taint_upper_capacity_threshold_percent must be less than scale_up_threshold_percent")

	checkThat(nodegroup.MinNodes < nodegroup.MaxNodes, "min_nodes must be less than max_nodes")
	checkThat(nodegroup.MaxNodes > 0, "max_nodes must be larger than 0")
	checkThat(nodegroup.SlowNodeRemovalRate <= nodegroup.FastNodeRemovalRate, "slow_node_removal_rate must be less than fast_node_removal_rate")

	checkThat(len(nodegroup.SoftDeleteGracePeriod) > 0, "soft_delete_grace_period must not be empty")
	checkThat(len(nodegroup.HardDeleteGracePeriod) >= 0, "hard_delete_grace_period must not be empty")

	checkThat(nodegroup.SoftDeleteGracePeriodDuration() > 0, "soft_delete_grace_period failed to parse into a time.Duration. check your formatting.")
	checkThat(nodegroup.HardDeleteGracePeriodDuration() >= 0, "hard_delete_grace_period failed to parse into a time.Duration. check your formatting.")
	checkThat(nodegroup.SoftDeleteGracePeriodDuration() < nodegroup.HardDeleteGracePeriodDuration() || nodegroup.HardDeleteGracePeriodDuration() == 0, "hard_delete_grace_period must be greater than soft_delete_grace_period or 0")

	checkThat(len(nodegroup.ScaleUpCoolDownPeriod) > 0, "scale_up_cool_down_period must not be empty")
	checkThat(nodegroup.ScaleUpCoolDownPeriodDuration() > 0, "soft_delete_grace_period failed to parse into a time.Duration. check your formatting.")

	// We use a custom validation function for taint_selection_method as it is a bit more of a complex option
	problems = append(problems, validateTaintSelectionMethods(nodegroup.TaintSelectionMethods)...)

	return problems
}

func validateTaintSelectionMethods(methods map[string]float64) (problems []error) {
	// Validate length
	if len(methods) == 0 {
		problems = append(problems, errors.New("taint_selection_methods must have at least 1 value"))
	}

	// Validate keys
	keys := make([]string, 0, len(methods))
	for key := range methods {
		keys = append(keys, key)
	}
	if !stringsInSlice(keys, TaintSelectionMethods) {
		problems = append(problems, fmt.Errorf("taint_selection_methods can only contain keys in %v", TaintSelectionMethods))
	}

	// Validate values
	total := float64(0)
	for _, value := range methods {
		total += value
	}
	if total != float64(1) {
		problems = append(problems, errors.New("taint_selection_methods weights should add up to 1.0"))
	}

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

// ShouldHardDelete returns whether nodes in the node group should be hard deleted
// when the hard delete grace period ends or kept forever
func (n *NodeGroupOptions) ShouldHardDelete() bool {
	return n.HardDeleteGracePeriodDuration() > 0
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
		// filter out daemonsets
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
		if pod.Spec.Affinity != nil && pod.Spec.Affinity.NodeAffinity != nil && pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
			if pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms != nil {
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
		}

		return false
	}
}

// NewPodDefaultFilterFunc creates a new PodFilterFunc that includes pods that do not have a selector
func NewPodDefaultFilterFunc() k8s.PodFilterFunc {
	return func(pod *v1.Pod) bool {
		// filter out daemonsets
		if k8s.PodIsDaemonSet(pod) {
			return false
		}

		// filter out static pods
		if k8s.PodIsStatic(pod) {
			return false
		}

		// Only include pods that pass the following:
		// - Don't have a nodeSelector
		// - Don't have an affinity
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

type nodeGroupsStateOpts struct {
	nodeGroups             []NodeGroupOptions
	client                 Client
	cloudProviderNodeGroup map[string]cloudprovider.NodeGroup
}

// BuildNodeGroupsState builds a node group state
func BuildNodeGroupsState(opts nodeGroupsStateOpts) map[string]*NodeGroupState {
	nodeGroupsState := make(map[string]*NodeGroupState)
	for _, ng := range opts.nodeGroups {
		nodeGroupsState[ng.Name] = &NodeGroupState{
			Opts:                   ng,
			NodeGroupLister:        opts.client.Listers[ng.Name],
			CloudProviderNodeGroup: opts.cloudProviderNodeGroup[ng.Name],
			// Setup the scaleLock timeouts for this nodegroup
			scaleUpLock: scaleLock{
				minimumLockDuration: ng.ScaleUpCoolDownPeriodDuration(),
			},
		}
	}
	return nodeGroupsState
}
