package controller

import (
	"testing"

	"github.com/atlassian/escalator/pkg/k8s"
	"github.com/atlassian/escalator/pkg/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestCalcScaleUpDeltaBelowThreshold(t *testing.T) {
	type args struct {
		pods              []*v1.Pod
		initialNodeAmount int
		nodeOpts          test.NodeOpts
		nodeGroup         *NodeGroupState
	}

	tests := []struct {
		name string
		args args
	}{
		{
			"test below threshold",
			args{
				test.BuildTestPods(10, test.PodOpts{
					CPU: []int64{500},
					Mem: []int64{100},
				}),
				2,
				test.NodeOpts{
					CPU: 1000,
					Mem: 4000,
				},
				&NodeGroupState{
					Opts: NodeGroupOptions{
						ScaleUpThresholdPercent: 70,
					},
				},
			},
		},
		{
			"test below threshold",
			args{
				test.BuildTestPods(10, test.PodOpts{
					CPU: []int64{500},
					Mem: []int64{2000},
				}),
				2,
				test.NodeOpts{
					CPU: 3000,
					Mem: 1000,
				},
				&NodeGroupState{
					Opts: NodeGroupOptions{
						ScaleUpThresholdPercent: 70,
					},
				},
			},
		},
		{
			"test below threshold",
			args{
				test.BuildTestPods(10, test.PodOpts{
					CPU: []int64{500},
					Mem: []int64{2000},
				}),
				2,
				test.NodeOpts{
					CPU: 3000,
					Mem: 1000,
				},
				&NodeGroupState{
					Opts: NodeGroupOptions{
						ScaleUpThresholdPercent: 40,
					},
				},
			},
		},
		{
			"test below threshold",
			args{
				test.BuildTestPods(10, test.PodOpts{
					CPU: []int64{500},
					Mem: []int64{2000},
				}),
				2,
				test.NodeOpts{
					CPU: 3000,
					Mem: 1000,
				},
				&NodeGroupState{
					Opts: NodeGroupOptions{
						ScaleUpThresholdPercent: 23,
					},
				},
			},
		},
		{
			"test below threshold",
			args{
				test.BuildTestPods(10, test.PodOpts{
					CPU: []int64{500},
					Mem: []int64{2000},
				}),
				2,
				test.NodeOpts{
					CPU: 3000,
					Mem: 1000,
				},
				&NodeGroupState{
					Opts: NodeGroupOptions{
						ScaleUpThresholdPercent: 3,
					},
				},
			},
		},
		{
			"test below threshold",
			args{
				test.BuildTestPods(80, test.PodOpts{
					CPU: []int64{1000},
					Mem: []int64{1000},
				}),
				100,
				test.NodeOpts{
					CPU: 1000,
					Mem: 1000,
				},
				&NodeGroupState{
					Opts: NodeGroupOptions{
						ScaleUpThresholdPercent: 70,
					},
				},
			},
		},
		{
			"test below threshold",
			args{
				test.BuildTestPods(150, test.PodOpts{
					CPU: []int64{1000},
					Mem: []int64{1000},
				}),
				100,
				test.NodeOpts{
					CPU: 1000,
					Mem: 1000,
				},
				&NodeGroupState{
					Opts: NodeGroupOptions{
						ScaleUpThresholdPercent: 110,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate percentage usage
			nodes := test.BuildTestNodes(tt.args.initialNodeAmount, tt.args.nodeOpts)
			cpuPercent, memPercent, _ := calculatePercentageUsage(tt.args.pods, nodes)

			// get pods requests
			memRequest, cpuRequest, err := k8s.CalculatePodsRequestsTotal(tt.args.pods)
			require.NoError(t, err)
			// Calculate scale up delta
			want, _ := calcScaleUpDelta(nodes, cpuPercent, memPercent, cpuRequest, memRequest, tt.args.nodeGroup)

			if want <= 0 {
				return
			}

			// Add the scale up delta amount of nodes and see if the new total of nodes will
			// bring it below the scale up threshold
			newNodes := append(nodes, test.BuildTestNodes(want, tt.args.nodeOpts)...)

			// Calculate the scale up percentage after adding the new nodes
			// Both of the percentages should be below the scale up threshold percent
			newCPUPercent, newMemPercent, _ := calculatePercentageUsage(tt.args.pods, newNodes)

			threshold := float64(tt.args.nodeGroup.Opts.ScaleUpThresholdPercent)
			assert.True(t, newCPUPercent <= threshold, "New CPU percent: %v should be less than threshold: %v", newCPUPercent, threshold)
			assert.True(t, newMemPercent <= threshold, "New Mem percent: %v should be less than threshold: %v", newMemPercent, threshold)
		})
	}

}

// Helper function for calculating percentage usage
func calculatePercentageUsage(pods []*v1.Pod, nodes []*v1.Node) (float64, float64, error) {
	// Calculate requests and capacity
	memRequest, cpuRequest, _ := k8s.CalculatePodsRequestsTotal(pods)
	memCapacity, cpuCapacity, _ := k8s.CalculateNodesCapacityTotal(nodes)

	// Calculate percentage usage
	return calcPercentUsage(cpuRequest, memRequest, cpuCapacity, memCapacity, int64(len(nodes)))
}

func TestCalcPercentUsage(t *testing.T) {
	type args struct {
		cpuRequest             resource.Quantity
		memRequest             resource.Quantity
		cpuCapacity            resource.Quantity
		memCapacity            resource.Quantity
		numberOfUntaintedNodes int64
	}
	tests := []struct {
		name        string
		args        args
		expectedCPU float64
		expectedMem float64
		err         error
	}{
		{
			"basic test",
			args{
				*resource.NewMilliQuantity(50, resource.DecimalSI),
				*resource.NewQuantity(50, resource.DecimalSI),
				*resource.NewMilliQuantity(100, resource.DecimalSI),
				*resource.NewQuantity(100, resource.DecimalSI),
				1,
			},
			50,
			50,
			nil,
		},
		{
			"divide by zero test",
			args{
				*resource.NewMilliQuantity(50, resource.DecimalSI),
				*resource.NewQuantity(50, resource.DecimalSI),
				*resource.NewMilliQuantity(0, resource.DecimalSI),
				*resource.NewQuantity(0, resource.DecimalSI),
				10,
			},
			0,
			0,
			errors.New("cannot divide by zero in percent calculation"),
		},
		{
			"no pods request while number of nodes is not 0",
			args{
				*resource.NewMilliQuantity(0, resource.DecimalSI),
				*resource.NewQuantity(0, resource.DecimalSI),
				*resource.NewMilliQuantity(0, resource.DecimalSI),
				*resource.NewQuantity(0, resource.DecimalSI),
				1,
			},
			0,
			0,
			errors.New("cannot divide by zero in percent calculation"),
		},
		{
			"zero numerator test",
			args{
				*resource.NewMilliQuantity(0, resource.DecimalSI),
				*resource.NewQuantity(0, resource.DecimalSI),
				*resource.NewMilliQuantity(66, resource.DecimalSI),
				*resource.NewQuantity(66, resource.DecimalSI),
				1,
			},
			0,
			0,
			nil,
		},
		{
			"zero all test",
			args{
				*resource.NewMilliQuantity(0, resource.DecimalSI),
				*resource.NewQuantity(0, resource.DecimalSI),
				*resource.NewMilliQuantity(0, resource.DecimalSI),
				*resource.NewQuantity(0, resource.DecimalSI),
				0,
			},
			0,
			0,
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpu, mem, err := calcPercentUsage(tt.args.cpuRequest, tt.args.memRequest, tt.args.cpuCapacity, tt.args.memCapacity, tt.args.numberOfUntaintedNodes)
			if tt.err == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, tt.err, err.Error())
			}
			assert.Equal(t, tt.expectedCPU, cpu)
			assert.Equal(t, tt.expectedMem, mem)
		})
	}
}
