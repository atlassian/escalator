package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
		c := Controller{
			Opts: &Opts{
				DryMode: tt.args.master,
			},
		}
		assert.Equal(t, c.dryMode(tt.args.nodeGroup), tt.want)
	}
}
