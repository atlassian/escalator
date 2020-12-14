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
		{
			Name:     "example",
			MinNodes: 3,
			MaxNodes: 6,
			DryMode:  false,
		},
		{
			Name:     "default",
			MinNodes: 0,
			MaxNodes: 6,
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
					nodeGroupsState["example"],
					2,
				},
			},
			2,
			false,
			"",
		},
		{
			"test try taint 4, min nodes = 3, total nodes = 6",
			args{
				scaleOpts{
					nodes,
					[]*v1.Node{},
					nodes,
					nodeGroupsState["example"],
					4,
				},
			},
			3,
			false,
			"",
		},
		{
			"test try taint 4, min nodes = 3, total nodes = 2",
			args{
				scaleOpts{
					nodes[:2],
					[]*v1.Node{},
					nodes[:2],
					nodeGroupsState["example"],
					4,
				},
			},
			0,
			true,
			"the number of nodes(2) is less than specified minimum of 3. Taking no action",
		},
		{
			"test try taint 4, min nodes = 0, total nodes = 3",
			args{
				scaleOpts{
					nodes[:3],
					[]*v1.Node{},
					nodes[:3],
					nodeGroupsState["default"],
					4,
				},
			},
			3,
			false,
			"",
		},
		{
			"test try taint 4, min nodes = 0, total nodes = 6",
			args{
				scaleOpts{
					nodes,
					[]*v1.Node{},
					nodes,
					nodeGroupsState["default"],
					4,
				},
			},
			4,
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
					_, err := k8s.DeleteToBeRemovedTaint(node, client)
					require.NoError(t, err)
					<-updateChan
				}
			}
			nodeGroupsState["example"].taintTracker = nil
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
			"first 3 nodes. taint 3",
			args{
				nodes[:3],
				nodeGroupsState["example"],
				3,
			},
			[]int{1, 2, 0},
		},
		{
			"first 3 nodes. taint 2",
			args{
				nodes[:3],
				nodeGroupsState["example"],
				2,
			},
			[]int{1, 2},
		},
		{
			"6 nodes. taint 0",
			args{
				nodes,
				nodeGroupsState["example"],
				0,
			},
			[]int{},
		},
		{
			"6 nodes. taint 2",
			args{
				nodes,
				nodeGroupsState["example"],
				2,
			},
			[]int{4, 5},
		},
		{
			"6 nodes. taint 6",
			args{
				nodes,
				nodeGroupsState["example"],
				6,
			},
			[]int{4, 5, 1, 2, 0, 3},
		},
		{
			"6 nodes. taint 5",
			args{
				nodes,
				nodeGroupsState["example"],
				5,
			},
			[]int{4, 5, 1, 2, 0},
		},
		{
			"6 nodes. taint 7",
			args{
				nodes,
				nodeGroupsState["example"],
				7,
			},
			[]int{4, 5, 1, 2, 0, 3},
		},
		{
			"4 nodes. taint 1",
			args{
				nodes[:4],
				nodeGroupsState["example"],
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
			got := c.taintOldestN(tt.args.nodes, tt.args.nodeGroup, tt.args.n)
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
			c.Opts.DryMode = true
			got = c.taintOldestN(tt.args.nodes, tt.args.nodeGroup, tt.args.n)
			assert.Equal(t, tt.want, got)

			// untaint all
			for _, node := range nodes {
				if _, tainted := k8s.GetToBeRemovedTaint(node); tainted {
					_, err := k8s.DeleteToBeRemovedTaint(node, client)
					require.NoError(t, err)
					<-updateChan
				}
			}
			nodeGroupsState["example"].taintTracker = nil
		})
	}
}

func TestControllerScaleDown(t *testing.T) {
	t.Skip("test not implemented")
}

func TestController_TryRemoveTaintedNodes(t *testing.T) {

	minNodes := 10
	maxNodes := 20
	nodeGroup := NodeGroupOptions{
		Name:                    "default",
		CloudProviderGroupName:  "default",
		MinNodes:                minNodes,
		MaxNodes:                maxNodes,
		ScaleUpThresholdPercent: 100,
	}
	nodeGroups := []NodeGroupOptions{nodeGroup}

	nodes := test.BuildTestNodes(10, test.NodeOpts{
		CPU:     1000,
		Mem:     1000,
		Tainted: true,
	})

	pods := buildTestPods(10, 1000, 1000)
	client, opts, err := buildTestClient(nodes, pods, nodeGroups, ListerOptions{})
	require.NoError(t, err)

	// For these test cases we only use 1 node group/cloud provider node group
	nodeGroupSize := 1

	// Create a test (mock) cloud provider
	testCloudProvider := test.NewCloudProvider(nodeGroupSize)
	testNodeGroup := test.NewNodeGroup(
		nodeGroup.CloudProviderGroupName,
		nodeGroup.Name,
		int64(minNodes),
		int64(maxNodes),
		int64(len(nodes)),
	)

	testCloudProvider.RegisterNodeGroup(testNodeGroup)

	// Create a node group state with the mapping of node groups to the cloud providers node groups
	nodeGroupsState := BuildNodeGroupsState(nodeGroupsStateOpts{
		nodeGroups: nodeGroups,
		client:     *client,
	})

	nodeGroupsState[testNodeGroup.ID()].NodeInfoMap = k8s.CreateNodeNameToInfoMap(pods, nodes)

	c := &Controller{
		Client:        client,
		Opts:          opts,
		stopChan:      nil,
		nodeGroups:    nodeGroupsState,
		cloudProvider: testCloudProvider,
	}

	// taint the oldest N according to the controller
	taintedIndex := c.taintOldestN(nodes, nodeGroupsState[testNodeGroup.ID()], 2)
	assert.Equal(t, len(taintedIndex), 2)

	// add the untainted the the untainted list
	taintedNodes := []*v1.Node{nodes[taintedIndex[0]], nodes[taintedIndex[1]]}
	var untaintedNodes []*v1.Node
	for i, n := range nodes {
		if n == taintedNodes[0] || n == taintedNodes[1] {
			continue
		}

		untaintedNodes = append(untaintedNodes, nodes[i])
	}
	assert.Equal(t, len(nodes)-2, len(untaintedNodes))

	tests := []struct {
		name                 string
		opts                 scaleOpts
		annotateFirstTainted bool
		want                 int
		wantErr              bool
	}{
		{
			"test normal delete all tainted",
			scaleOpts{
				nodes,
				taintedNodes,
				untaintedNodes,
				nodeGroupsState[testNodeGroup.ID()],
				0, // not used in TryRemoveTaintedNodes
			},
			false,
			-2,
			false,
		},
		{
			"test normal skip first tainted",
			scaleOpts{
				nodes,
				taintedNodes,
				untaintedNodes,
				nodeGroupsState[testNodeGroup.ID()],
				0, // not used in TryRemoveTaintedNodes
			},
			true,
			-1,
			false,
		},
		{
			"test none tainted",
			scaleOpts{
				nodes,
				[]*v1.Node{},
				nodes,
				nodeGroupsState[testNodeGroup.ID()],
				0, // not used in TryRemoveTaintedNodes
			},
			false,
			0,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.annotateFirstTainted {
				tt.opts.taintedNodes[0].Annotations = map[string]string{
					NodeEscalatorIgnoreAnnotation: "skip for testing",
				}
			}
			got, err := c.TryRemoveTaintedNodes(tt.opts)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
