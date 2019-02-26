package controller

import (
	"testing"

	"github.com/atlassian/escalator/pkg/test"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
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
					Opts: NodeGroupOptions{
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
					Opts: NodeGroupOptions{
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
					Opts: NodeGroupOptions{
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
					Opts: NodeGroupOptions{
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
			c := &Controller{
				Opts: Opts{
					DryMode: tt.args.master,
				},
			}
			assert.Equal(t, c.dryMode(tt.args.nodeGroup), tt.want)
		})
	}
}

func TestControllerFilterNodes(t *testing.T) {
	nodes := []*v1.Node{
		0: test.BuildTestNode(test.NodeOpts{
			Name:    "n1",
			Tainted: true,
		}),
		1: test.BuildTestNode(test.NodeOpts{
			Name:    "n2",
			Tainted: false,
		}),
		2: test.BuildTestNode(test.NodeOpts{
			Name:    "n3",
			Tainted: true,
		}),
		3: test.BuildTestNode(test.NodeOpts{
			Name:    "n4",
			Tainted: false,
		}),
		4: test.BuildTestNode(test.NodeOpts{
			Name:    "n5",
			Tainted: true,
		}),
		5: test.BuildTestNode(test.NodeOpts{
			Name:    "n6",
			Tainted: false,
		}),
	}

	type args struct {
		nodeGroup *NodeGroupState
		allNodes  []*v1.Node
		master    bool
	}
	tests := []struct {
		name               string
		args               args
		wantUntaintedNodes []*v1.Node
		wantTaintedNodes   []*v1.Node
		wantCordonedNodes  []*v1.Node
	}{
		{
			"basic filter not drymode",
			args{
				&NodeGroupState{
					Opts: NodeGroupOptions{
						DryMode: false,
					},
				},
				nodes,
				false,
			},
			[]*v1.Node{nodes[1], nodes[3], nodes[5]},
			[]*v1.Node{nodes[0], nodes[2], nodes[4]},
			[]*v1.Node{},
		},
		{
			"basic filter drymode",
			args{
				&NodeGroupState{
					Opts: NodeGroupOptions{
						DryMode: true,
					},
					taintTracker: []string{"n1", "n3", "n5"},
				},
				nodes,
				true,
			},
			[]*v1.Node{nodes[1], nodes[3], nodes[5]},
			[]*v1.Node{nodes[0], nodes[2], nodes[4]},
			[]*v1.Node{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				Opts: Opts{
					DryMode: tt.args.master,
				},
			}
			gotUntaintedNodes, gotTaintedNodes, gotCordonedNodes := c.filterNodes(tt.args.nodeGroup, tt.args.allNodes)
			assert.Equal(t, tt.wantUntaintedNodes, gotUntaintedNodes)
			assert.Equal(t, tt.wantTaintedNodes, gotTaintedNodes)
			assert.Equal(t, tt.wantCordonedNodes, gotCordonedNodes)
		})
	}
}
