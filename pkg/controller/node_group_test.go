package controller_test

import (
	"strings"
	"testing"
	"time"

	"github.com/atlassian/escalator/pkg/controller"
	"github.com/atlassian/escalator/pkg/test"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
)

func TestNewPodLabelFilterFunc(t *testing.T) {
	buildengPod := test.BuildTestPod(test.PodOpts{
		NodeSelectorKey:   "customer",
		NodeSelectorValue: "buildeng",
	})
	badKeyPod := test.BuildTestPod(test.PodOpts{
		NodeSelectorKey:   "wronglabelkey",
		NodeSelectorValue: "buildeng",
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
		NodeSelectorValue: "buildeng",
		Owner:             "DaemonSet",
	})
	affinity := test.BuildTestPod(test.PodOpts{
		NodeAffinityKey:   "customer",
		NodeAffinityValue: "buildeng",
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
			"buildeng customer should pass",
			args{
				"customer",
				"buildeng",
				buildengPod,
			},
			true,
		},
		{
			"buildeng customer should fail",
			args{
				"customer",
				"kitt",
				buildengPod,
			},
			false,
		},
		{
			"buildeng wrong key should fail",
			args{
				"customer",
				"buildeng",
				badKeyPod,
			},
			false,
		},
		{
			"buildeng wrong value should fail",
			args{
				"customer",
				"buildeng",
				badLabelPod,
			},
			false,
		},
		{
			"buildeng bad both should fail",
			args{
				"customer",
				"buildeng",
				badBothPod,
			},
			false,
		},
		{
			"daemonset should be false",
			args{
				"customer",
				"buildeng",
				daemonSet,
			},
			false,
		},
		{
			"correct affinty should be true",
			args{
				"customer",
				"buildeng",
				affinity,
			},
			true,
		},
		{
			"wrong affinty should be false",
			args{
				"customer",
				"shared",
				affinity,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := controller.NewPodAffinityFilterFunc(tt.args.labelKey, tt.args.labelValue)
			got := f(tt.args.pod)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewPodDefaultFilterFunc(t *testing.T) {
	buildengPod := test.BuildTestPod(test.PodOpts{
		NodeSelectorKey:   "customer",
		NodeSelectorValue: "buildeng",
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
			"buildeng customer should fail",
			args{
				buildengPod,
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
			f := controller.NewPodDefaultFilterFunc()
			got := f(tt.args.pod)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewNodeLabelFilterFunc(t *testing.T) {
	buildengNode := test.BuildTestNode(test.NodeOpts{
		LabelKey:   "customer",
		LabelValue: "buildeng",
	})
	badKeyNode := test.BuildTestNode(test.NodeOpts{
		LabelKey:   "wronglabelkey",
		LabelValue: "buildeng",
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
			"buildeng customer should pass",
			args{
				"customer",
				"buildeng",
				buildengNode,
			},
			true,
		},
		{
			"buildeng customer should fail",
			args{
				"customer",
				"kitt",
				buildengNode,
			},
			false,
		},
		{
			"buildeng wrong key should fail",
			args{
				"customer",
				"buildeng",
				badKeyNode,
			},
			false,
		},
		{
			"buildeng wrong value should fail",
			args{
				"customer",
				"buildeng",
				badLabelNode,
			},
			false,
		},
		{
			"buildeng bad both should fail",
			args{
				"customer",
				"buildeng",
				badBothNode,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := controller.NewNodeLabelFilterFunc(tt.args.labelKey, tt.args.labelValue)
			got := f(tt.args.node)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUnmarshalNodeGroupOptions(t *testing.T) {
	t.Run("test yaml unmarshal good", func(t *testing.T) {
		yamlReader := strings.NewReader(yaml)
		opts, err := controller.UnmarshalNodeGroupOptions(yamlReader)

		assert.NoError(t, err)
		assert.Equal(t, 2, len(opts))
		assert.NotNil(t, opts[0])
		assert.Equal(t, "buildeng", opts[0].Name)
		assert.Equal(t, "customer", opts[0].LabelKey)
		assert.Equal(t, "buildeng", opts[0].LabelValue)
		assert.Equal(t, 5, opts[0].MinNodes)
		assert.Equal(t, 300, opts[0].MaxNodes)
		assert.Equal(t, true, opts[0].DryMode)
		assert.Equal(t, "10m", opts[0].SoftDeleteGracePeriod)
		assert.Equal(t, time.Minute*10, opts[0].SoftDeleteGracePeriodDuration())
		assert.Equal(t, time.Duration(0), opts[0].HardDeleteGracePeriodDuration())

		assert.NotNil(t, opts[1])
		assert.Equal(t, "default", opts[1].Name)
		assert.Equal(t, "customer", opts[1].LabelKey)
		assert.Equal(t, "shared", opts[1].LabelValue)
		assert.Equal(t, 1, opts[1].MinNodes)
		assert.Equal(t, 10, opts[1].MaxNodes)
		assert.Equal(t, true, opts[1].DryMode)
	})

	t.Run("test yaml unmarshal bad", func(t *testing.T) {
		yamlReader := strings.NewReader(yamlErr)
		opts, err := controller.UnmarshalNodeGroupOptions(yamlReader)

		assert.Error(t, err)
		assert.Empty(t, opts)
	})

	t.Run("test yaml unmarshal Buildeng good", func(t *testing.T) {
		yamlReader := strings.NewReader(yamlBE)
		opts, err := controller.UnmarshalNodeGroupOptions(yamlReader)

		assert.NoError(t, err)
		assert.Equal(t, 1, len(opts))
		assert.NotNil(t, opts[0])
		assert.Equal(t, "buildeng", opts[0].Name)
		assert.Equal(t, "customer", opts[0].LabelKey)
		assert.Equal(t, "buildeng", opts[0].LabelValue)
		assert.Equal(t, 10, opts[0].MinNodes)
		assert.Equal(t, 300, opts[0].MaxNodes)
		assert.Equal(t, false, opts[0].DryMode)
	})
}

var yamlErr = `
- name: 4
node_groups:
`

var yaml = `
node_groups:
  - name: "buildeng"
    label_key: "customer"
    label_value: "buildeng"
    min_nodes: 5
    max_nodes: 300
    dry_mode: true
    taint_upper_capacity_threshhold_percent: 70
    taint_lower_capacity_threshhold_percent: 50
    untaint_upper_capacity_threshhold_percent: 95
    untaint_lower_capacity_threshhold_percent: 90
    slow_node_removal_rate: 2
    fast_node_removal_rate: 3
    slow_node_revival_rate: 2
    fast_node_revival_rate: 3
    soft_delete_grace_period: 10m
    hard_delete_grace_period: 42
    scale_up_cooldown_period: 1h2m30s
  - name: "default"
    label_key: "customer"
    label_value: "shared"
    min_nodes: 1
    max_nodes: 10
    dry_mode: true
    taint_upper_capacity_threshhold_percent: 25
    taint_lower_capacity_threshhold_percent: 20
    untaint_upper_capacity_threshhold_percent: 45
    untaint_lower_capacity_threshhold_percent: 30
    slow_node_removal_rate: 2
    fast_node_removal_rate: 3
    slow_node_revival_rate: 2
    fast_node_revival_rate: 3
    scale_up_cooldown_period: 21h
`

var yamlBE = `node_groups:
  - name: "buildeng"
    label_key: "customer"
    label_value: "buildeng"
    min_nodes: 10
    max_nodes: 300
    dry_mode: false
    taint_upper_capacity_threshhold_percent: 70
    taint_lower_capacity_threshhold_percent: 45
    untaint_upper_capacity_threshhold_percent: 95
    untaint_lower_capacity_threshhold_percent: 90
    slow_node_removal_rate: 2
    fast_node_removal_rate: 5
    slow_node_revival_rate: 2
    fast_node_revival_rate: 10`

func TestValidateNodeGroup(t *testing.T) {
	type args struct {
		nodegroup controller.NodeGroupOptions
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			"valid nodegroup",
			args{
				controller.NodeGroupOptions{
					Name:                                "test",
					LabelKey:                            "customer",
					LabelValue:                          "buileng",
					CloudProviderASG:                    "somegroup",
					TaintUpperCapacityThreshholdPercent: 70,
					TaintLowerCapacityThreshholdPercent: 60,
					ScaleUpThreshholdPercent:            100,
					MinNodes:                            1,
					MaxNodes:                            3,
					SlowNodeRemovalRate:                 1,
					FastNodeRemovalRate:                 2,
					SoftDeleteGracePeriod:               "10m",
					HardDeleteGracePeriod:               "1h10m",
					ScaleUpCoolDownPeriod:               "55m",
				},
			},
			nil,
		},
		{
			"invalid nodegroup",
			args{
				controller.NodeGroupOptions{
					Name:                                "",
					LabelKey:                            "customer",
					LabelValue:                          "buileng",
					CloudProviderASG:                    "somegroup",
					TaintUpperCapacityThreshholdPercent: 70,
					TaintLowerCapacityThreshholdPercent: 90,
					ScaleUpThreshholdPercent:            100,
					MinNodes:                            1,
					MaxNodes:                            0,
					SlowNodeRemovalRate:                 1,
					FastNodeRemovalRate:                 2,
					SoftDeleteGracePeriod:               "10",
					HardDeleteGracePeriod:               "1h10m",
					ScaleUpCoolDownPeriod:               "21h21m21s",
				},
			},
			[]string{
				"name cannot be empty",
				"lower taint threshhold must be lower than upper taint threshold",
				"min nodes must be smaller than max nodes",
				"soft grace period failed to parse into a time.Duration. check your formatting.",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := controller.ValidateNodeGroup(tt.args.nodegroup)
			eq := assert.Equal(t, len(tt.want), len(errs))
			if eq && errs != nil {
				for i, err := range errs {
					assert.Equal(t, tt.want[i], err.Error())
				}
			}
		})
	}
}
