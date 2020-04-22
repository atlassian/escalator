package controller

import (
	"fmt"
	"io"
	"time"

	"github.com/atlassian/escalator/pkg/cloudprovider/aws"
	"github.com/atlassian/escalator/pkg/k8s"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	v1lister "k8s.io/client-go/listers/core/v1"
)

// DefaultNodeGroup is used for any pods that don't have a node selector defined
const DefaultNodeGroup = "default"

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

	TaintEffect v1.TaintEffect `json:"taint_effect,omitempty" yaml:"taint_effect,omitempty"`

	AWS AWSNodeGroupOptions `json:"aws" yaml:"aws"`

	// Private variables for storing the parsed duration from the string
	softDeleteGracePeriodDuration time.Duration
	hardDeleteGracePeriodDuration time.Duration
	scaleUpCoolDownPeriodDuration time.Duration
}

// AWSNodeGroupOptions represents a nodegroup running on a cluster that is
// using the AWS cloud provider
type AWSNodeGroupOptions struct {
	LaunchTemplateID          string   `json:"launch_template_id,omitempty" yaml:"launch_template_id,omitempty"`
	LaunchTemplateVersion     string   `json:"launch_template_version,omitempty" yaml:"launch_template_version,omitempty"`
	FleetInstanceReadyTimeout string   `json:"fleet_instance_ready_timeout,omitempty" yaml:"fleet_instance_ready_timeout,omitempty"`
	Lifecycle                 string   `json:"lifecycle,omitempty" yaml:"lifecycle,omitempty"`
	InstanceTypeOverrides     []string `json:"instance_type_overrides,omitempty" yaml:"instance_type_overrides,omitempty"`

	// Private variables for storing the parsed duration from the string
	fleetInstanceReadyTimeout time.Duration
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

	// Allow exclusion of the MinNodes and MaxNodes options so that we can "auto discover" them from the cloud provider
	if !nodegroup.autoDiscoverMinMaxNodeOptions() {
		checkThat(nodegroup.MinNodes < nodegroup.MaxNodes, "min_nodes must be less than max_nodes")
		checkThat(nodegroup.MaxNodes > 0, "max_nodes must be larger than 0")
		checkThat(nodegroup.MinNodes >= 0, "min_nodes must be not less than 0")
	}

	checkThat(nodegroup.SlowNodeRemovalRate <= nodegroup.FastNodeRemovalRate, "slow_node_removal_rate must be less than fast_node_removal_rate")

	checkThat(len(nodegroup.SoftDeleteGracePeriod) > 0, "soft_delete_grace_period must not be empty")
	checkThat(len(nodegroup.HardDeleteGracePeriod) > 0, "hard_delete_grace_period must not be empty")

	checkThat(nodegroup.SoftDeleteGracePeriodDuration() > 0, "soft_delete_grace_period failed to parse into a time.Duration. check your formatting.")
	checkThat(nodegroup.HardDeleteGracePeriodDuration() > 0, "hard_delete_grace_period failed to parse into a time.Duration. check your formatting.")
	checkThat(nodegroup.SoftDeleteGracePeriodDuration() < nodegroup.HardDeleteGracePeriodDuration(), "soft_delete_grace_period must be less than hard_delete_grace_period")

	checkThat(len(nodegroup.ScaleUpCoolDownPeriod) > 0, "scale_up_cool_down_period must not be empty")
	checkThat(nodegroup.ScaleUpCoolDownPeriodDuration() > 0, "soft_delete_grace_period failed to parse into a time.Duration. check your formatting.")

	checkThat(validTaintEffect(nodegroup.TaintEffect), "taint_effect must be valid kubernetes taint")

	checkThat(validAWSLifecycle(nodegroup.AWS.Lifecycle), "aws.lifecycle must be '%v' or '%v' if provided.", aws.LifecycleOnDemand, aws.LifecycleSpot)
	return problems
}

// Lifecycle must be either on-demand or spot if it's provided. An empty string is allowed to preserve backwards compatibility
func validAWSLifecycle(lifecycle string) bool {
	return len(lifecycle) == 0 || lifecycle == aws.LifecycleOnDemand || lifecycle == aws.LifecycleSpot
}

// Empty String is valid value for TaintEffect as AddToBeRemovedTaint method will default to NoSchedule
func validTaintEffect(taintEffect v1.TaintEffect) bool {
	return len(taintEffect) == 0 || k8s.TaintEffectTypes[taintEffect]
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

// autoDiscoverMinMaxNodeOptions returns whether the min_nodes and max_nodes options should be "auto-discovered" from the cloud provider
func (n *NodeGroupOptions) autoDiscoverMinMaxNodeOptions() bool {
	return n.MinNodes == 0 && n.MaxNodes == 0
}

// FleetInstanceReadyTimeoutDuration lazily returns/parses the fleetInstanceReadyTimeout string into a duration
func (n *AWSNodeGroupOptions) FleetInstanceReadyTimeoutDuration() time.Duration {
	if n.fleetInstanceReadyTimeout == 0 && n.FleetInstanceReadyTimeout != "" {
		duration, err := time.ParseDuration(n.FleetInstanceReadyTimeout)
		if err != nil {
			return 0
		}
		n.fleetInstanceReadyTimeout = duration
	} else if n.fleetInstanceReadyTimeout == 0 && n.FleetInstanceReadyTimeout == "" {
		n.fleetInstanceReadyTimeout = 1 * time.Minute
	}

	return n.fleetInstanceReadyTimeout
}

// NodeGroupLister is just a light wrapper around a pod lister and node lister
// Used for grouping a nodegroup and their listers
type NodeGroupLister struct {
	// Pod lister
	Pods k8s.PodLister
	// Node lister
	Nodes k8s.NodeLister
}

// unwrapNodeSelectorTerms is a helper to safely get the NodeSelectorTerms array from a pod
// returns nil slice if not exists
func unwrapNodeSelectorTerms(pod *v1.Pod) []v1.NodeSelectorTerm {
	if pod.Spec.Affinity != nil &&
		pod.Spec.Affinity.NodeAffinity != nil &&
		pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		return pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
	}
	return nil
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
		for _, term := range unwrapNodeSelectorTerms(pod) {
			for _, expression := range term.MatchExpressions {
				// this is the key we're looking for
				if expression.Key != labelKey {
					continue
				}
				// perform the appropriate match for the expression operator
				// we only support In
				if expression.Operator == v1.NodeSelectorOpIn {
					for _, value := range expression.Values {
						if value == labelValue {
							return true
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
	nodeGroups []NodeGroupOptions
	client     Client
}

// BuildNodeGroupsState builds a node group state
func BuildNodeGroupsState(opts nodeGroupsStateOpts) map[string]*NodeGroupState {
	nodeGroupsState := make(map[string]*NodeGroupState)
	for _, ng := range opts.nodeGroups {
		nodeGroupsState[ng.Name] = &NodeGroupState{
			Opts:            ng,
			NodeGroupLister: opts.client.Listers[ng.Name],
			// Setup the scaleLock timeouts for this nodegroup
			scaleUpLock: scaleLock{
				minimumLockDuration: ng.ScaleUpCoolDownPeriodDuration(),
				nodegroup:           ng.Name,
			},
		}
	}
	return nodeGroupsState
}
