package controller

import (
	"strings"
	"testing"
	"time"

	"github.com/atlassian/escalator/pkg/test"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

func TestNewPodLabelFilterFunc(t *testing.T) {
	examplePod := test.BuildTestPod(test.PodOpts{
		NodeSelectorKey:   "customer",
		NodeSelectorValue: "example",
	})
	badKeyPod := test.BuildTestPod(test.PodOpts{
		NodeSelectorKey:   "wronglabelkey",
		NodeSelectorValue: "example",
	})
	badLabelPod := test.BuildTestPod(test.PodOpts{
		NodeSelectorKey:   "customer",
		NodeSelectorValue: "wronglabelkey",
	})
	badBothPod := test.BuildTestPod(test.PodOpts{
		NodeSelectorKey:   "wronglabelkey",
		NodeSelectorValue: "wronglabelkey",
	})
	daemonSet := test.BuildTestPod(test.PodOpts{
		NodeSelectorKey:   "customer",
		NodeSelectorValue: "example",
		Owner:             "DaemonSet",
	})
	affinity := test.BuildTestPod(test.PodOpts{
		NodeAffinityKey:   "customer",
		NodeAffinityValue: "example",
	})
	affinityIncorrectOp := test.BuildTestPod(test.PodOpts{
		NodeAffinityKey:   "customer",
		NodeAffinityValue: "example",
		NodeAffinityOp:    v1.NodeSelectorOpNotIn,
	})

	type args struct {
		labelKey   string
		labelValue string
		pod        *v1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"example customer should pass",
			args{
				"customer",
				"example",
				examplePod,
			},
			true,
		},
		{
			"example customer should fail",
			args{
				"customer",
				"kitt",
				examplePod,
			},
			false,
		},
		{
			"example wrong key should fail",
			args{
				"customer",
				"example",
				badKeyPod,
			},
			false,
		},
		{
			"example wrong value should fail",
			args{
				"customer",
				"example",
				badLabelPod,
			},
			false,
		},
		{
			"example bad both should fail",
			args{
				"customer",
				"example",
				badBothPod,
			},
			false,
		},
		{
			"daemonset should be false",
			args{
				"customer",
				"example",
				daemonSet,
			},
			false,
		},
		{
			"correct affinity should be true",
			args{
				"customer",
				"example",
				affinity,
			},
			true,
		},
		{
			"wrong affinity should be false",
			args{
				"customer",
				"shared",
				affinity,
			},
			false,
		},
		{
			"correct affinity wrong operator should be false",
			args{
				"customer",
				"shared",
				affinityIncorrectOp,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewPodAffinityFilterFunc(tt.args.labelKey, tt.args.labelValue)
			got := f(tt.args.pod)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewPodDefaultFilterFunc(t *testing.T) {
	examplePod := test.BuildTestPod(test.PodOpts{
		NodeSelectorKey:   "customer",
		NodeSelectorValue: "example",
	})
	sharedPod := test.BuildTestPod(test.PodOpts{
		NodeSelectorKey:   "customer",
		NodeSelectorValue: "shared",
	})
	podKey := test.BuildTestPod(test.PodOpts{
		NodeSelectorKey: "customer",
	})
	podValue := test.BuildTestPod(test.PodOpts{
		NodeSelectorValue: "shared",
	})
	noSelector := test.BuildTestPod(test.PodOpts{})
	daemonSet := test.BuildTestPod(test.PodOpts{
		Owner: "DaemonSet",
	})
	affinity := test.BuildTestPod(test.PodOpts{
		NodeAffinityKey:   "customer",
		NodeAffinityValue: "shared",
	})

	type args struct {
		pod *v1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"example customer should fail",
			args{
				examplePod,
			},
			false,
		},
		{
			"shared customer should fail",
			args{
				sharedPod,
			},
			false,
		},
		{
			"pod key only should fail",
			args{
				podKey,
			},
			false,
		},
		{
			"pod value only should fail",
			args{
				podValue,
			},
			false,
		},
		{
			"no selector should pass",
			args{
				noSelector,
			},
			true,
		},
		{
			"daemonset should fail",
			args{
				daemonSet,
			},
			false,
		},
		{
			"node affinity fail",
			args{
				affinity,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewPodDefaultFilterFunc()
			got := f(tt.args.pod)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewNodeLabelFilterFunc(t *testing.T) {
	exampleNode := test.BuildTestNode(test.NodeOpts{
		LabelKey:   "customer",
		LabelValue: "example",
	})
	badKeyNode := test.BuildTestNode(test.NodeOpts{
		LabelKey:   "wronglabelkey",
		LabelValue: "example",
	})
	badLabelNode := test.BuildTestNode(test.NodeOpts{
		LabelKey:   "customer",
		LabelValue: "wronglabelkey",
	})
	badBothNode := test.BuildTestNode(test.NodeOpts{
		LabelKey:   "wronglabelkey",
		LabelValue: "wronglabelkey",
	})

	type args struct {
		labelKey   string
		labelValue string
		node       *v1.Node
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"example customer should pass",
			args{
				"customer",
				"example",
				exampleNode,
			},
			true,
		},
		{
			"example customer should fail",
			args{
				"customer",
				"kitt",
				exampleNode,
			},
			false,
		},
		{
			"example wrong key should fail",
			args{
				"customer",
				"example",
				badKeyNode,
			},
			false,
		},
		{
			"example wrong value should fail",
			args{
				"customer",
				"example",
				badLabelNode,
			},
			false,
		},
		{
			"example bad both should fail",
			args{
				"customer",
				"example",
				badBothNode,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewNodeLabelFilterFunc(tt.args.labelKey, tt.args.labelValue)
			got := f(tt.args.node)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUnmarshalNodeGroupOptions(t *testing.T) {
	t.Run("test yaml unmarshal good", func(t *testing.T) {
		yamlReader := strings.NewReader(yamlValid)
		opts, err := UnmarshalNodeGroupOptions(yamlReader)

		assert.NoError(t, err)
		assert.Equal(t, 2, len(opts))
		assert.NotNil(t, opts[0])
		assert.Equal(t, "example", opts[0].Name)
		assert.Equal(t, "customer", opts[0].LabelKey)
		assert.Equal(t, "example", opts[0].LabelValue)
		assert.Equal(t, 5, opts[0].MinNodes)
		assert.Equal(t, 300, opts[0].MaxNodes)
		assert.Equal(t, true, opts[0].DryMode)
		assert.Equal(t, "10m", opts[0].SoftDeleteGracePeriod)
		assert.Equal(t, time.Minute*10, opts[0].SoftDeleteGracePeriodDuration())
		assert.Equal(t, time.Duration(0), opts[0].HardDeleteGracePeriodDuration())
		assert.Equal(t, v1.TaintEffectNoExecute, opts[0].TaintEffect)

		assert.NotNil(t, opts[1])
		assert.Equal(t, "default", opts[1].Name)
		assert.Equal(t, "customer", opts[1].LabelKey)
		assert.Equal(t, "shared", opts[1].LabelValue)
		assert.Equal(t, 1, opts[1].MinNodes)
		assert.Equal(t, 10, opts[1].MaxNodes)
		assert.Equal(t, true, opts[1].DryMode)
		assert.Equal(t, v1.TaintEffectNoSchedule, opts[1].TaintEffect)
	})

	t.Run("test yaml unmarshal bad", func(t *testing.T) {
		yamlReader := strings.NewReader(yamlErr)
		opts, err := UnmarshalNodeGroupOptions(yamlReader)

		assert.Error(t, err)
		assert.Empty(t, opts)
	})

	t.Run("test yaml unmarshal example good", func(t *testing.T) {
		yamlReader := strings.NewReader(yamlBE)
		opts, err := UnmarshalNodeGroupOptions(yamlReader)

		assert.NoError(t, err)
		assert.Equal(t, 1, len(opts))
		assert.NotNil(t, opts[0])
		assert.Equal(t, "example", opts[0].Name)
		assert.Equal(t, "customer", opts[0].LabelKey)
		assert.Equal(t, "example", opts[0].LabelValue)
		assert.Equal(t, 10, opts[0].MinNodes)
		assert.Equal(t, 300, opts[0].MaxNodes)
		assert.Equal(t, false, opts[0].DryMode)
		assert.Empty(t, opts[0].TaintEffect)
	})
}

var yamlErr = `
- name: 4
node_groups:
`

var yamlValid = `
node_groups:
  - name: "example"
    label_key: "customer"
    label_value: "example"
    min_nodes: 5
    max_nodes: 300
    dry_mode: true
    taint_upper_capacity_threshold_percent: 70
    taint_lower_capacity_threshold_percent: 50
    untaint_upper_capacity_threshold_percent: 95
    untaint_lower_capacity_threshold_percent: 90
    slow_node_removal_rate: 2
    fast_node_removal_rate: 3
    soft_delete_grace_period: 10m
    hard_delete_grace_period: 42
    scale_up_cooldown_period: 1h2m30s
    taint_effect: NoExecute
  - name: "default"
    label_key: "customer"
    label_value: "shared"
    min_nodes: 1
    max_nodes: 10
    dry_mode: true
    taint_upper_capacity_threshold_percent: 25
    taint_lower_capacity_threshold_percent: 20
    slow_node_removal_rate: 2
    fast_node_removal_rate: 3
    scale_up_cooldown_period: 21h
    taint_effect: NoSchedule
`

var yamlBE = `node_groups:
  - name: "example"
    label_key: "customer"
    label_value: "example"
    min_nodes: 10
    max_nodes: 300
    dry_mode: false
    taint_upper_capacity_threshold_percent: 70
    taint_lower_capacity_threshold_percent: 45
    slow_node_removal_rate: 2
    fast_node_removal_rate: 5`

func TestValidateNodeGroup(t *testing.T) {
	type args struct {
		nodegroup NodeGroupOptions
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			"valid nodegroup",
			args{
				NodeGroupOptions{
					Name:                               "test",
					LabelKey:                           "customer",
					LabelValue:                         "buileng",
					CloudProviderGroupName:             "somegroup",
					TaintUpperCapacityThresholdPercent: 70,
					TaintLowerCapacityThresholdPercent: 60,
					ScaleUpThresholdPercent:            100,
					MinNodes:                           1,
					MaxNodes:                           3,
					SlowNodeRemovalRate:                1,
					FastNodeRemovalRate:                2,
					SoftDeleteGracePeriod:              "10m",
					HardDeleteGracePeriod:              "1h10m",
					ScaleUpCoolDownPeriod:              "55m",
					TaintEffect:                        "NoExecute",
				},
			},
			nil,
		},
		{
			"valid nodegroup with empty TaintEffect",
			args{
				NodeGroupOptions{
					Name:                               "test",
					LabelKey:                           "customer",
					LabelValue:                         "buileng",
					CloudProviderGroupName:             "somegroup",
					TaintUpperCapacityThresholdPercent: 70,
					TaintLowerCapacityThresholdPercent: 60,
					ScaleUpThresholdPercent:            100,
					MinNodes:                           1,
					MaxNodes:                           3,
					SlowNodeRemovalRate:                1,
					FastNodeRemovalRate:                2,
					SoftDeleteGracePeriod:              "10m",
					HardDeleteGracePeriod:              "1h10m",
					ScaleUpCoolDownPeriod:              "55m",
					TaintEffect:                        "",
				},
			},
			nil,
		},
		{
			"invalid nodegroup",
			args{
				NodeGroupOptions{
					Name:                               "",
					LabelKey:                           "customer",
					LabelValue:                         "buileng",
					CloudProviderGroupName:             "somegroup",
					TaintUpperCapacityThresholdPercent: 70,
					TaintLowerCapacityThresholdPercent: 90,
					ScaleUpThresholdPercent:            100,
					MinNodes:                           1,
					MaxNodes:                           0,
					SlowNodeRemovalRate:                1,
					FastNodeRemovalRate:                2,
					SoftDeleteGracePeriod:              "10",
					HardDeleteGracePeriod:              "1h10m",
					ScaleUpCoolDownPeriod:              "21h21m21s",
					TaintEffect:                        "invalid",
				},
			},
			[]string{
				"name cannot be empty",
				"taint_lower_capacity_threshold_percent must be less than taint_upper_capacity_threshold_percent",
				"min_nodes must be less than max_nodes",
				"max_nodes must be larger than 0",
				"soft_delete_grace_period failed to parse into a time.Duration. check your formatting.",
				"taint_effect must be valid kubernetes taint",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateNodeGroup(tt.args.nodegroup)
			eq := assert.Equal(t, len(tt.want), len(errs))
			if eq && errs != nil {
				for i, err := range errs {
					assert.Equal(t, tt.want[i], err.Error())
				}
			}
		})
	}
}

func TestNodeGroupOptions_autoDiscoverMinMaxNodeOptions(t *testing.T) {
	options := NodeGroupOptions{MinNodes: 1, MaxNodes: 6}
	assert.False(t, options.autoDiscoverMinMaxNodeOptions())

	optionsAutoDiscover := NodeGroupOptions{MinNodes: 0, MaxNodes: 0}
	assert.True(t, optionsAutoDiscover.autoDiscoverMinMaxNodeOptions())
}
