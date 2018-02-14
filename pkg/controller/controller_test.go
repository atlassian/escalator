package controller

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/atlassian/escalator/pkg/k8s"
	"github.com/atlassian/escalator/pkg/test"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
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

func TestControllerTaintOldestN(t *testing.T) {

	nodes := []*v1.Node{
		0: test.BuildTestNode(test.NodeOpts{
			Name:     "n1",
			Creation: time.Date(2011, 3, 3, 13, 0, 0, 0, time.UTC),
		}),
		1: test.BuildTestNode(test.NodeOpts{
			Name:     "n2",
			Creation: time.Date(2009, 3, 3, 12, 0, 0, 0, time.UTC),
		}),
		2: test.BuildTestNode(test.NodeOpts{
			Name:     "n3",
			Creation: time.Date(2010, 3, 3, 13, 0, 0, 0, time.UTC),
		}),
		3: test.BuildTestNode(test.NodeOpts{
			Name:     "n4",
			Creation: time.Date(2015, 3, 3, 13, 0, 0, 0, time.UTC),
		}),
		4: test.BuildTestNode(test.NodeOpts{
			Name:     "n5",
			Creation: time.Date(2005, 3, 3, 13, 0, 0, 0, time.UTC),
		}),
		5: test.BuildTestNode(test.NodeOpts{
			Name:     "n6",
			Creation: time.Date(2007, 3, 3, 13, 0, 0, 0, time.UTC),
		}),
	}

	nodeGroups := []NodeGroupOptions{
		NodeGroupOptions{
			Name:     "buildeng",
			MinNodes: 1,
			MaxNodes: 5,
			DryMode:  false,
		},
	}
	nodeGroupsState := make(map[string]*NodeGroupState)
	for _, ng := range nodeGroups {
		nodeGroupsState[ng.Name] = &NodeGroupState{
			Opts: ng,
		}
	}

	fakeClient, updateChan := test.BuildFakeClient(nodes, []*v1.Pod{})
	opts := Opts{
		K8SClient:    fakeClient,
		NodeGroups:   nodeGroups,
		ScanInterval: 1 * time.Minute,
		DryMode:      false,
	}
	client := &Client{
		Interface: fakeClient,
	}

	type args struct {
		nodes     []*v1.Node
		nodeGroup *NodeGroupState
		n         int
	}
	tests := []struct {
		name string
		args args
		want []int
	}{
		{
			"first 3 nodes. taint 3",
			args{
				nodes[:3],
				nodeGroupsState["buildeng"],
				3,
			},
			[]int{1, 2, 0},
		},
		{
			"first 3 nodes. taint 2",
			args{
				nodes[:3],
				nodeGroupsState["buildeng"],
				2,
			},
			[]int{1, 2},
		},
		{
			"6 nodes. taint 0",
			args{
				nodes,
				nodeGroupsState["buildeng"],
				0,
			},
			[]int{},
		},
		{
			"6 nodes. taint 2",
			args{
				nodes,
				nodeGroupsState["buildeng"],
				2,
			},
			[]int{4, 5},
		},
		{
			"6 nodes. taint 6",
			args{
				nodes,
				nodeGroupsState["buildeng"],
				6,
			},
			[]int{4, 5, 1, 2, 0, 3},
		},
		{
			"6 nodes. taint 5",
			args{
				nodes,
				nodeGroupsState["buildeng"],
				5,
			},
			[]int{4, 5, 1, 2, 0},
		},
		{
			"6 nodes. taint 7",
			args{
				nodes,
				nodeGroupsState["buildeng"],
				7,
			},
			[]int{4, 5, 1, 2, 0, 3},
		},
		{
			"4 nodes. taint 1",
			args{
				nodes[:4],
				nodeGroupsState["buildeng"],
				1,
			},
			[]int{1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				Client:     client,
				Opts:       opts,
				stopChan:   nil,
				nodeGroups: nodeGroupsState,
			}
			// test wet mode
			c.Opts.DryMode = false
			assert.NoError(t, k8s.BeginTaintFailSafe(len(tt.want)))
			got := c.taintOldestN(tt.args.nodes, tt.args.nodeGroup, tt.args.n)
			assert.NoError(t, k8s.EndTaintFailSafe(len(got)))
			eq := assert.Equal(t, tt.want, got)
			if eq {
				for _, i := range got {
					updated := test.NameFromChan(updateChan, 1*time.Second)
					t.Run(fmt.Sprintf("checking %v returned node drymode off", i), func(t *testing.T) {
						// test that the node was actually tainted
						if eq := assert.Equal(t, tt.args.nodes[i].Name, updated); eq {
							_, tainted := k8s.GetToBeRemovedTaint(tt.args.nodes[i])
							assert.True(t, tainted)
						}
					})
				}
			}

			// test dry mode
			assert.NoError(t, k8s.BeginTaintFailSafe(len(tt.want)))
			c.Opts.DryMode = true
			got = c.taintOldestN(tt.args.nodes, tt.args.nodeGroup, tt.args.n)
			assert.NoError(t, k8s.EndTaintFailSafe(len(got)))
			assert.Equal(t, tt.want, got)

			// untaint all
			for _, node := range nodes {
				if _, tainted := k8s.GetToBeRemovedTaint(node); tainted {
					k8s.DeleteToBeRemovedTaint(node, client)
					<-updateChan
				}
			}
			nodeGroupsState["buildeng"].taintTracker = nil
		})
	}
}

func TestControllerUntaintNewestN(t *testing.T) {

	nodes := []*v1.Node{
		0: test.BuildTestNode(test.NodeOpts{
			Name:     "n1",
			Creation: time.Date(2011, 3, 3, 13, 0, 0, 0, time.UTC),
		}),
		1: test.BuildTestNode(test.NodeOpts{
			Name:     "n2",
			Creation: time.Date(2009, 3, 3, 12, 0, 0, 0, time.UTC),
		}),
		2: test.BuildTestNode(test.NodeOpts{
			Name:     "n3",
			Creation: time.Date(2010, 3, 3, 13, 0, 0, 0, time.UTC),
		}),
		3: test.BuildTestNode(test.NodeOpts{
			Name:     "n4",
			Creation: time.Date(2015, 3, 3, 13, 0, 0, 0, time.UTC),
		}),
		4: test.BuildTestNode(test.NodeOpts{
			Name:     "n5",
			Creation: time.Date(2005, 3, 3, 13, 0, 0, 0, time.UTC),
		}),
		5: test.BuildTestNode(test.NodeOpts{
			Name:     "n6",
			Creation: time.Date(2007, 3, 3, 13, 0, 0, 0, time.UTC),
		}),
	}

	nodeGroups := []NodeGroupOptions{
		NodeGroupOptions{
			Name:     "buildeng",
			MinNodes: 1,
			MaxNodes: 5,
			DryMode:  false,
		},
	}
	nodeGroupsState := make(map[string]*NodeGroupState)
	for _, ng := range nodeGroups {
		nodeGroupsState[ng.Name] = &NodeGroupState{
			Opts: ng,
		}
	}

	fakeClient, updateChan := test.BuildFakeClient(nodes, []*v1.Pod{})
	opts := Opts{
		K8SClient:    fakeClient,
		NodeGroups:   nodeGroups,
		ScanInterval: 1 * time.Minute,
		DryMode:      false,
	}
	client := &Client{
		Interface: fakeClient,
	}

	type args struct {
		nodes     []*v1.Node
		nodeGroup *NodeGroupState
		n         int
	}
	tests := []struct {
		name string
		args args
		want []int
	}{
		{
			"first 3 nodes. untaint 3",
			args{
				nodes[:3],
				nodeGroupsState["buildeng"],
				3,
			},
			[]int{0, 2, 1},
		},
		{
			"first 3 nodes. untaint 2",
			args{
				nodes[:3],
				nodeGroupsState["buildeng"],
				2,
			},
			[]int{0, 2},
		},
		{
			"6 nodes. untaint 0",
			args{
				nodes,
				nodeGroupsState["buildeng"],
				0,
			},
			[]int{},
		},
		{
			"6 nodes. untaint 2",
			args{
				nodes,
				nodeGroupsState["buildeng"],
				2,
			},
			[]int{3, 0},
		},
		{
			"6 nodes. untaint 6",
			args{
				nodes,
				nodeGroupsState["buildeng"],
				6,
			},
			[]int{3, 0, 2, 1, 5, 4},
		},
		{
			"6 nodes. untaint 5",
			args{
				nodes,
				nodeGroupsState["buildeng"],
				5,
			},
			[]int{3, 0, 2, 1, 5},
		},
		{
			"6 nodes. untaint 7",
			args{
				nodes,
				nodeGroupsState["buildeng"],
				7,
			},
			[]int{3, 0, 2, 1, 5, 4},
		},
		{
			"4 nodes. untaint 1",
			args{
				nodes[:4],
				nodeGroupsState["buildeng"],
				1,
			},
			[]int{3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				Client:     client,
				Opts:       opts,
				stopChan:   nil,
				nodeGroups: nodeGroupsState,
			}

			// taint all
			k8s.BeginTaintFailSafe(len(nodes))
			var tc int
			for _, node := range nodes {
				if _, tainted := k8s.GetToBeRemovedTaint(node); !tainted {
					k8s.AddToBeRemovedTaint(node, client)
					nodeGroupsState["buildeng"].taintTracker = append(nodeGroupsState["buildeng"].taintTracker, node.Name)
					<-updateChan
					tc++
				}
			}
			k8s.EndTaintFailSafe(tc)

			// test wet mode
			c.Opts.DryMode = false
			got := c.untaintNewestN(tt.args.nodes, tt.args.nodeGroup, tt.args.n)
			eq := assert.Equal(t, tt.want, got)
			if eq {
				for _, i := range got {
					updated := test.NameFromChan(updateChan, 1*time.Second)
					t.Run(fmt.Sprintf("checking %v returned node drymode off", i), func(t *testing.T) {
						assert.Equal(t, tt.args.nodes[i].Name, updated)
						// test that the node is actually untainted
						if eq := assert.Equal(t, tt.args.nodes[i].Name, updated); eq {
							_, tainted := k8s.GetToBeRemovedTaint(tt.args.nodes[i])
							assert.False(t, tainted)
						}
					})
				}
			}

			// test dry mode
			c.Opts.DryMode = true
			got = c.untaintNewestN(tt.args.nodes, tt.args.nodeGroup, tt.args.n)
			assert.Equal(t, tt.want, got)
		})
	}
}
