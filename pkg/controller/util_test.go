package controller

import (
	"errors"
	"github.com/atlassian/escalator/pkg/test"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"testing"
)

func TestCalcNodeWorth(t *testing.T) {
	nodes := test.BuildTestNodes(200, test.NodeOpts{})

	tests := []struct {
		name  string
		nodes []*v1.Node
		want  float64
		err   error
	}{
		{
			"10 nodes",
			nodes[:10],
			10.0,
			nil,
		},
		{
			"50 nodes",
			nodes[:50],
			2.0,
			nil,
		},
		{
			"100 nodes",
			nodes[:100],
			1.0,
			nil,
		},
		{
			"200 nodes",
			nodes[:200],
			0.5,
			nil,
		},
		{
			"0 nodes",
			make([]*v1.Node, 0),
			0,
			errors.New("cannot divide by zero in percent calculation"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, err := calcNodeWorth(tt.nodes)
			assert.Equal(t, tt.err, err)
			assert.Equal(t, tt.want, want)
		})
	}

}

func TestCalcScaleUpDelta(t *testing.T) {
	type args struct {
		nodes []*v1.Node
		cpuPercent float64
		memPercent float64
		nodeGroup *NodeGroupState
	}

	nodes := test.BuildTestNodes(100, test.NodeOpts{})

	tests := []struct {
		name  string
		args  args
		want int
		err   error
	}{
		{
			"scale up 10%",
			args{
				nodes,
				80,
				80,
				&NodeGroupState{
					Opts: NodeGroupOptions{
						ScaleUpThreshholdPercent: 70,
					},
				},
			},
			10,
			nil,
		},
		{
			"scale up 10% cpu",
			args{
				nodes,
				80,
				50,
				&NodeGroupState{
					Opts: NodeGroupOptions{
						ScaleUpThreshholdPercent: 70,
					},
				},
			},
			10,
			nil,
		},
		{
			"scale up 20% mem",
			args{
				nodes,
				80,
				90,
				&NodeGroupState{
					Opts: NodeGroupOptions{
						ScaleUpThreshholdPercent: 70,
					},
				},
			},
			20,
			nil,
		},
		{
			"scale up 30% cpu",
			args{
				nodes,
				80,
				50,
				&NodeGroupState{
					Opts: NodeGroupOptions{
						ScaleUpThreshholdPercent: 50,
					},
				},
			},
			30,
			nil,
		},
		{
			"scale up with zero nodes",
			args{
				make([]*v1.Node, 0),
				80,
				50,
				&NodeGroupState{
					Opts: NodeGroupOptions{
						ScaleUpThreshholdPercent: 70,
					},
				},
			},
			0,
			errors.New("cannot divide by zero in percent calculation"),
		},
		{
			"test with weird values",
			args{
				nodes,
				100,
				100,
				&NodeGroupState{
					Opts: NodeGroupOptions{
						ScaleUpThreshholdPercent: 100,
					},
				},
			},
			0,
			nil,
		},
		{
			"test with weird values",
			args{
				nodes,
				100,
				100,
				&NodeGroupState{
					Opts: NodeGroupOptions{
						ScaleUpThreshholdPercent: 0,
					},
				},
			},
			100,
			nil,
		},
		{
			"test with weird values",
			args{
				nodes,
				0,
				0,
				&NodeGroupState{
					Opts: NodeGroupOptions{
						ScaleUpThreshholdPercent: 70,
					},
				},
			},
			-70,
			errors.New("negative scale up delta"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, err := calcScaleUpDelta(tt.args.nodes, tt.args.cpuPercent, tt.args.memPercent, tt.args.nodeGroup)
			assert.Equal(t, tt.err, err)
			assert.Equal(t, tt.want, want)
		})
	}
}

func TestCalcPercentUsage(t *testing.T) {
	type args struct {
		cpuR resource.Quantity
		memR resource.Quantity
		cpuA resource.Quantity
		memA resource.Quantity
	}
	tests := []struct {
		name string
		args args
		cpu  float64
		mem  float64
		err  error
	}{
		{
			"basic test",
			args{
				*resource.NewMilliQuantity(50, resource.DecimalSI),
				*resource.NewQuantity(50, resource.DecimalSI),
				*resource.NewMilliQuantity(100, resource.DecimalSI),
				*resource.NewQuantity(100, resource.DecimalSI),
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
			},
			0,
			0,
			errors.New("Cannot divide by zero in percent calculation"),
		},
		{
			"zero numerator test",
			args{
				*resource.NewMilliQuantity(0, resource.DecimalSI),
				*resource.NewQuantity(0, resource.DecimalSI),
				*resource.NewMilliQuantity(66, resource.DecimalSI),
				*resource.NewQuantity(66, resource.DecimalSI),
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
			},
			0,
			0,
			errors.New("Cannot divide by zero in percent calculation"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpu, mem, err := calcPercentUsage(tt.args.cpuR, tt.args.memR, tt.args.cpuA, tt.args.memA)
			assert.Equal(t, tt.err, err)
			assert.Equal(t, tt.cpu, cpu)
			assert.Equal(t, tt.mem, mem)
		})
	}
}
