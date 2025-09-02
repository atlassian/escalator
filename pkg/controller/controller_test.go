package controller

import (
	"context"
	"testing"
	"time"

	"github.com/atlassian/escalator/pkg/k8s"
	"github.com/atlassian/escalator/pkg/test"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		6: test.BuildTestNode(test.NodeOpts{
			Name:         "n7",
			ForceTainted: true,
		}),
	}

	type args struct {
		nodeGroup *NodeGroupState
		allNodes  []*v1.Node
		master    bool
	}
	tests := []struct {
		name                  string
		args                  args
		wantUntaintedNodes    []*v1.Node
		wantTaintedNodes      []*v1.Node
		wantForceTaintedNodes []*v1.Node
		wantCordonedNodes     []*v1.Node
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
			[]*v1.Node{nodes[6]},
			[]*v1.Node{},
		},
		{
			"basic filter drymode",
			args{
				&NodeGroupState{
					Opts: NodeGroupOptions{
						DryMode: true,
					},
					taintTracker:      []string{"n1", "n3", "n5"},
					forceTaintTracker: []string{"n7"},
				},
				nodes,
				true,
			},
			[]*v1.Node{nodes[1], nodes[3], nodes[5]},
			[]*v1.Node{nodes[0], nodes[2], nodes[4]},
			[]*v1.Node{nodes[6]},
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
			gotUntaintedNodes, gotTaintedNodes, gotForceTaintedNodes, gotCordonedNodes := c.filterNodes(tt.args.nodeGroup, tt.args.allNodes)
			assert.Equal(t, tt.wantUntaintedNodes, gotUntaintedNodes)
			assert.Equal(t, tt.wantTaintedNodes, gotTaintedNodes)
			assert.Equal(t, tt.wantForceTaintedNodes, gotForceTaintedNodes)
			assert.Equal(t, tt.wantCordonedNodes, gotCordonedNodes)
		})
	}
}

func TestGetNodesOrderedNewestFirst(t *testing.T) {
	c := &Controller{}
	now := time.Now()

	nodes := []*v1.Node{
		test.BuildTestNode(test.NodeOpts{Name: "n1", Creation: now.Add(-20 * time.Minute)}),
		test.BuildTestNode(test.NodeOpts{Name: "n2", Creation: now.Add(-5 * time.Minute)}),
		test.BuildTestNode(test.NodeOpts{Name: "n3", Creation: now.Add(-15 * time.Minute)}),
	}

	reversedNodes := c.getNodesOrderedNewestFirst(nodes)

	assert.Equal(t, "n2", reversedNodes[0].Name)
	assert.Equal(t, "n3", reversedNodes[1].Name)
	assert.Equal(t, "n1", reversedNodes[2].Name)
}

func TestFilterOutNodesTooNew(t *testing.T) {
	c := &Controller{}
	now := time.Now()

	type args struct {
		state *NodeGroupState
		nodes []*v1.Node
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			"filter out new nodes",
			args{
				&NodeGroupState{
					Opts: NodeGroupOptions{
						unhealthyNodeGracePeriodDuration: 10 * time.Minute,
					},
				},
				[]*v1.Node{
					test.BuildTestNode(test.NodeOpts{Name: "n1", Creation: now.Add(-5 * time.Minute)}),
					// The threshold needs to be exceeded for the instance to be considered unhealthy
					test.BuildTestNode(test.NodeOpts{Name: "n2", Creation: now.Add(-10 * time.Minute)}),
					test.BuildTestNode(test.NodeOpts{Name: "n3", Creation: now.Add(-15 * time.Minute)}),
					test.BuildTestNode(test.NodeOpts{Name: "n4", Creation: now.Add(-20 * time.Minute)}),
				},
			},
			[]string{"n2", "n3", "n4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.filterOutNodesTooNew(tt.args.state, tt.args.nodes)
			gotNames := make([]string, len(got))

			for i, node := range got {
				gotNames[i] = node.Name
			}

			assert.Equal(t, tt.want, gotNames)
		})
	}
}

func TestGetMostRecentNodes(t *testing.T) {
	c := &Controller{}
	now := time.Now()

	nodes1 := []*v1.Node{
		test.BuildTestNode(test.NodeOpts{Name: "n1", Creation: now.Add(-5 * time.Minute)}),
		test.BuildTestNode(test.NodeOpts{Name: "n2", Creation: now.Add(-15 * time.Minute)}),
		test.BuildTestNode(test.NodeOpts{Name: "n3", Creation: now.Add(-20 * time.Minute)}),
		test.BuildTestNode(test.NodeOpts{Name: "n4", Creation: now.Add(-30 * time.Minute)}),
	}

	nodes2 := []*v1.Node{
		test.BuildTestNode(test.NodeOpts{Name: "n1", Creation: now.Add(-5 * time.Minute)}),
		test.BuildTestNode(test.NodeOpts{Name: "n2", Creation: now.Add(-15 * time.Minute)}),
		test.BuildTestNode(test.NodeOpts{Name: "n3", Creation: now.Add(-20 * time.Minute)}),
	}

	type args struct {
		state *NodeGroupState
		nodes []*v1.Node
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			"get most recent 50% (even)",
			args{
				&NodeGroupState{
					Opts: NodeGroupOptions{
						HealthCheckNewestNodesPercent: 50,
					},
				},
				nodes1,
			},
			[]string{"n1", "n2"},
		},
		{
			"get most recent 49% (even)",
			args{
				&NodeGroupState{
					Opts: NodeGroupOptions{
						HealthCheckNewestNodesPercent: 49,
					},
				},
				nodes1,
			},
			[]string{"n1", "n2"},
		},
		{
			"get most recent 75% (even)",
			args{
				&NodeGroupState{
					Opts: NodeGroupOptions{
						HealthCheckNewestNodesPercent: 75,
					},
				},
				nodes1,
			},
			[]string{"n1", "n2", "n3"},
		},
		{
			"get most recent 50% (odd)",
			args{
				&NodeGroupState{
					Opts: NodeGroupOptions{
						HealthCheckNewestNodesPercent: 50,
					},
				},
				nodes2,
			},
			[]string{"n1", "n2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.getMostRecentNodes(tt.args.state, tt.args.nodes)
			gotNames := make([]string, len(got))

			for i, node := range got {
				gotNames[i] = node.Name
			}

			assert.Equal(t, tt.want, gotNames)
		})
	}
}

func TestCountUnhealthyNodes(t *testing.T) {
	c := &Controller{}
	now := time.Now()

	type args struct {
		state *NodeGroupState
		nodes []*v1.Node
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			"count unhealthy nodes",
			args{
				&NodeGroupState{
					Opts: NodeGroupOptions{
						unhealthyNodeGracePeriodDuration: 10 * time.Minute,
					},
				},
				[]*v1.Node{
					test.BuildTestNode(test.NodeOpts{Name: "n1", NotReady: true, Creation: now.Add(-5 * time.Minute)}),
					test.BuildTestNode(test.NodeOpts{Name: "n2", NotReady: true, Creation: now.Add(-15 * time.Minute)}),
					test.BuildTestNode(test.NodeOpts{Name: "n3", NotReady: false, Creation: now.Add(-20 * time.Minute)}),
					test.BuildTestNode(test.NodeOpts{Name: "n4", NotReady: true, Creation: now.Add(-30 * time.Minute)}),
					test.BuildTestNode(test.NodeOpts{Name: "n5", NotReady: true, Creation: now.Add(-30 * time.Minute), Unschedulable: true}),
				},
			},
			// n2 and n4 are unhealthy and old enough
			// n5 is not because it is cordoned and is not taken into account
			// escalator.
			2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.countUnhealthyNodes(tt.args.state, tt.args.nodes)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTaintUnhealthyInstances(t *testing.T) {
	now := time.Now()

	type args struct {
		nodes []*v1.Node
		state *NodeGroupState
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			"taint unhealthy instances",
			args{
				[]*v1.Node{
					test.BuildTestNode(test.NodeOpts{Name: "n1", NotReady: true, Creation: now.Add(-5 * time.Minute)}),
					test.BuildTestNode(test.NodeOpts{Name: "n2", NotReady: true, Creation: now.Add(-15 * time.Minute)}),
					test.BuildTestNode(test.NodeOpts{Name: "n3", NotReady: false, Creation: now.Add(-20 * time.Minute)}),
					test.BuildTestNode(test.NodeOpts{Name: "n4", NotReady: true, Creation: now.Add(-30 * time.Minute)}),
					test.BuildTestNode(test.NodeOpts{Name: "n5", NotReady: true, Creation: now.Add(-30 * time.Minute), Unschedulable: true}),
				},
				&NodeGroupState{
					Opts: NodeGroupOptions{
						unhealthyNodeGracePeriodDuration: 10 * time.Minute,
					},
				},
			},
			[]string{"n2", "n4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient, _ := test.BuildFakeClient(tt.args.nodes, nil)
			c := &Controller{
				Client: &Client{
					Interface: fakeClient,
				},
			}

			tt.args.state.taintTracker = []string{}

			nodeLister, _ := test.NewTestNodeWatcher(tt.args.nodes, test.NodeListerOptions{})
			podLister, _ := test.NewTestPodWatcher(nil, test.PodListerOptions{})
			tt.args.state.NodeGroupLister = NewNodeGroupLister(podLister, nodeLister, tt.args.state.Opts)

			taintedIndices := c.taintUnhealthyInstances(tt.args.nodes, tt.args.state)

			var taintedNames []string

			for _, index := range taintedIndices {
				taintedNames = append(taintedNames, tt.args.nodes[index].Name)
			}

			assert.ElementsMatch(t, tt.want, taintedNames)

			for _, name := range tt.want {
				updatedNode, err := c.Client.CoreV1().Nodes().Get(context.Background(), name, metav1.GetOptions{})
				assert.NoError(t, err)

				_, tainted := k8s.GetToBeRemovedTaint(updatedNode)
				assert.True(t, tainted)
			}
		})
	}
}
