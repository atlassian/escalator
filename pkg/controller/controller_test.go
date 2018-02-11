package controller

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestControllerDryMode(t *testing.T) {
	type args struct {
		nodeGroup *NodeGroupState
		master    bool
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"both true. dry mode returns true",
			args{
				&NodeGroupState{
					Opts: &NodeGroupOptions{
						DryMode: true,
					},
				},
				true,
			},
			true,
		},
		{
			"master true config false. dry mode returns true",
			args{
				&NodeGroupState{
					Opts: &NodeGroupOptions{
						DryMode: false,
					},
				},
				true,
			},
			true,
		},
		{
			"master false config true. dry mode returns true",
			args{
				&NodeGroupState{
					Opts: &NodeGroupOptions{
						DryMode: true,
					},
				},
				false,
			},
			true,
		},
		{
			"both false. dry mode returns false",
			args{
				&NodeGroupState{
					Opts: &NodeGroupOptions{
						DryMode: false,
					},
				},
				false,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Controller{
				Opts: &Opts{
					DryMode: tt.args.master,
				},
			}
			assert.Equal(t, c.dryMode(tt.args.nodeGroup), tt.want)
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
