package controller

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/atlassian/escalator/pkg/k8s"
	"github.com/atlassian/escalator/pkg/test"
	"k8s.io/api/core/v1"
)

func TestControllerScaleDownTaint(t *testing.T) {

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
			MinNodes: 3,
			MaxNodes: 6,
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
		opts scaleOpts
	}
	tests := []struct {
		name    string
		args    args
		want    int
		wantErr bool
		errStr  string
	}{
		{
			"test valid taint 2",
			args{
				scaleOpts{
					nodes,
					[]*v1.Node{},
					nodes,
					[]*v1.Pod{},
					nodeGroupsState["buildeng"],
					50,
					2,
				},
			},
			2,
			false,
			"",
		},
		{
			"test try taint 4, min nodes = 3",
			args{
				scaleOpts{
					nodes,
					[]*v1.Node{},
					nodes,
					[]*v1.Pod{},
					nodeGroupsState["buildeng"],
					50,
					4,
				},
			},
			3,
			false,
			"",
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
			tainted, err := c.scaleDownTaint(tt.args.opts)
			assert.Equal(t, tt.want, tainted)
			assert.Equal(t, tt.wantErr, err != nil)
			if tt.wantErr {
				assert.Equal(t, tt.errStr, err.Error())
			}
			for i := 0; i < tainted; i++ {
				<-updateChan
			}
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
