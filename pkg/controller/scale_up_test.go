package controller

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	"github.com/atlassian/escalator/pkg/k8s"
	"github.com/atlassian/escalator/pkg/test"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

func TestControllerScaleUpUntaint(t *testing.T) {
	t.Skip("test not implemented")
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
		{
			Name:     "example",
			MinNodes: 1,
			MaxNodes: 5,
			DryMode:  false,
		},
	}

	nodeGroupsState := BuildNodeGroupsState(nodeGroupsStateOpts{
		nodeGroups: nodeGroups,
	})

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
				nodeGroupsState["example"],
				3,
			},
			[]int{0, 2, 1},
		},
		{
			"first 3 nodes. untaint 2",
			args{
				nodes[:3],
				nodeGroupsState["example"],
				2,
			},
			[]int{0, 2},
		},
		{
			"6 nodes. untaint 0",
			args{
				nodes,
				nodeGroupsState["example"],
				0,
			},
			[]int{},
		},
		{
			"6 nodes. untaint 2",
			args{
				nodes,
				nodeGroupsState["example"],
				2,
			},
			[]int{3, 0},
		},
		{
			"6 nodes. untaint 6",
			args{
				nodes,
				nodeGroupsState["example"],
				6,
			},
			[]int{3, 0, 2, 1, 5, 4},
		},
		{
			"6 nodes. untaint 5",
			args{
				nodes,
				nodeGroupsState["example"],
				5,
			},
			[]int{3, 0, 2, 1, 5},
		},
		{
			"6 nodes. untaint 7",
			args{
				nodes,
				nodeGroupsState["example"],
				7,
			},
			[]int{3, 0, 2, 1, 5, 4},
		},
		{
			"4 nodes. untaint 1",
			args{
				nodes[:4],
				nodeGroupsState["example"],
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
			var tc int
			for _, node := range nodes {
				if _, tainted := k8s.GetToBeRemovedTaint(node); !tainted {
					_, err := k8s.AddToBeRemovedTaint(node, client, "NoSchedule")
					require.NoError(t, err)
					nodeGroupsState["example"].taintTracker = append(nodeGroupsState["example"].taintTracker, node.Name)
					<-updateChan
					tc++
				}
			}

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

func TestCalculateNodesToAdd(t *testing.T) {

	type args struct {
		nodesToAdd int64
		TargetSize int64
		MaxNodes   int64
	}

	tests := []struct {
		name string
		args args
		want int64
	}{
		{
			"Regular scale up",
			args{
				nodesToAdd: 10,
				TargetSize: 20,
				MaxNodes:   50,
			},
			int64(10),
		},
		{
			"Clamp to ASG ceiling",
			args{
				nodesToAdd: 45,
				TargetSize: 10,
				MaxNodes:   50,
			},
			int64(40),
		},
		{
			"Already scaled to maximum",
			args{
				nodesToAdd: 10,
				TargetSize: 50,
				MaxNodes:   50,
			},
			int64(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{}
			nodesToAdd := c.calculateNodesToAdd(tt.args.nodesToAdd, tt.args.TargetSize, tt.args.MaxNodes)
			assert.Equal(t, tt.want, nodesToAdd)
		})
	}
}
