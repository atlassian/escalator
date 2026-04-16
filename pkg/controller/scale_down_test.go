package controller

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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
			Name:                   "example",
			CloudProviderGroupName: "example-asg",
			MinNodes:               3,
			MaxNodes:               6,
			DryMode:                false,
		},
		{
			Name:                   "default",
			CloudProviderGroupName: "default-asg",
			MinNodes:               0,
			MaxNodes:               6,
			DryMode:                false,
		},
		{
			Name:                   "asg-constrained",
			CloudProviderGroupName: "asg-constrained-asg",
			MinNodes:               1, // Very permissive - ASG constraint should be more restrictive
			MaxNodes:               10,
			DryMode:                false,
		},
		{
			Name:                   "asg-atmin",
			CloudProviderGroupName: "asg-atmin-asg",
			MinNodes:               0, // No Escalator restriction - ASG constraint should kick in
			MaxNodes:               6,
			DryMode:                false,
		},
	}

	nodeGroupsState := BuildNodeGroupsState(nodeGroupsStateOpts{
		nodeGroups: nodeGroups,
	})

	testCloudProvider := test.NewCloudProvider(len(nodeGroups))

	exampleNodeGroup := test.NewNodeGroup(
		"example-asg",
		"example",
		3, // minSize
		6, // maxSize
		6, // targetSize (current desired capacity)
	)
	defaultNodeGroup := test.NewNodeGroup(
		"default-asg",
		"default",
		0, // minSize
		6, // maxSize
		6, // targetSize (current desired capacity)
	)

	asgConstrainedNodeGroup := test.NewNodeGroup(
		"asg-constrained-asg",
		"asg-constrained",
		4,  // minSize - this will be more restrictive than Escalator MinNodes
		10, // maxSize
		7,  // targetSize (allows some deletions: 7-4=3 max)
	)
	asgAtMinNodeGroup := test.NewNodeGroup(
		"asg-atmin-asg",
		"asg-atmin",
		3, // minSize
		6, // maxSize
		3, // targetSize = minSize (no deletions allowed)
	)

	testCloudProvider.RegisterNodeGroup(exampleNodeGroup)
	testCloudProvider.RegisterNodeGroup(defaultNodeGroup)
	testCloudProvider.RegisterNodeGroup(asgConstrainedNodeGroup)
	testCloudProvider.RegisterNodeGroup(asgAtMinNodeGroup)

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
		{
			"test ASG constraint: try taint 4 but ASG only allows 3 (Escalator MinNodes=1, ASG MinSize=4, TargetSize=7)",
			args{
				scaleOpts{
					nodes,
					[]*v1.Node{},
					[]*v1.Node{},
					nodes,
					nodeGroupsState["asg-constrained"], // Escalator MinNodes=1, ASG MinSize=4, TargetSize=7
					4,                                  // want to taint 4, but ASG constraint: maxDeletable = 7-4 = 3
				},
			},
			3, // should only taint 3 due to ASG constraint
			false,
			"",
		},
		{
			"test ASG constraint: ASG at minimum prevents tainting (Escalator MinNodes=0, ASG MinSize=3, TargetSize=3)",
			args{
				scaleOpts{
					nodes[:3], // use 3 nodes to match ASG current size
					[]*v1.Node{},
					[]*v1.Node{},
					nodes[:3],
					nodeGroupsState["asg-atmin"], // Escalator MinNodes=0, ASG MinSize=3, TargetSize=3
					2,                            // want to taint 2, but ASG constraint: maxDeletable = 3-3 = 0
				},
			},
			0, // should taint 0 due to ASG constraint (at minimum)
			false,
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				Client:        client,
				Opts:          opts,
				stopChan:      nil,
				nodeGroups:    nodeGroupsState,
				cloudProvider: testCloudProvider,
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

		// Make odd nodes unhealthy
		var ready = v1.NodeReady

		if i%2 == 0 {
			// This is used arbitrarily to mean "not ready"
			ready = v1.NodeNetworkUnavailable
		}

		for i, condition := range n.Status.Conditions {
			if condition.Type == v1.NodeReady {
				n.Status.Conditions[i].Type = ready
			}
		}

		untaintedNodes = append(untaintedNodes, nodes[i])
	}
	assert.Equal(t, len(nodes)-2, len(untaintedNodes))

	tests := []struct {
		name                           string
		opts                           scaleOpts
		annotateFirstTainted           bool
		healthyNodesAllowedToBeRemoved bool
		want                           int
		wantErr                        bool
	}{
		{
			"test normal delete all tainted",
			scaleOpts{
				nodes,
				taintedNodes,
				[]*v1.Node{},
				untaintedNodes,
				nodeGroupsState[testNodeGroup.ID()],
				0, // not used in TryRemoveTaintedNodes
			},
			false,
			true,
			-2,
			false,
		},
		{
			"test normal skip first tainted",
			scaleOpts{
				nodes,
				taintedNodes,
				[]*v1.Node{},
				untaintedNodes,
				nodeGroupsState[testNodeGroup.ID()],
				0, // not used in TryRemoveTaintedNodes
			},
			true,
			true,
			-1,
			false,
		},
		{
			"test none tainted",
			scaleOpts{
				nodes,
				[]*v1.Node{},
				[]*v1.Node{},
				nodes,
				nodeGroupsState[testNodeGroup.ID()],
				0, // not used in TryRemoveTaintedNodes
			},
			false,
			true,
			0,
			false,
		},
		{
			"test normal delete without unhealthy nodes",
			scaleOpts{
				nodes,
				taintedNodes,
				[]*v1.Node{},
				untaintedNodes,
				nodeGroupsState[testNodeGroup.ID()],
				0, // not used in TryRemoveTaintedNodes
			},
			false,
			false,
			-1,
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
			got, err := c.TryRemoveTaintedNodes(tt.opts, tt.healthyNodesAllowedToBeRemoved)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}

	forceTests := []struct {
		name                 string
		opts                 scaleOpts
		annotateFirstTainted bool
		want                 int
		wantErr              bool
	}{
		{
			"test none force tainted",
			scaleOpts{
				nodes,
				[]*v1.Node{},
				[]*v1.Node{},
				nodes,
				nodeGroupsState[testNodeGroup.ID()],
				0, // not used in TryRemoveTaintedNodes
			},
			false,
			0,
			false,
		},
		{
			"test one tainted",
			scaleOpts{
				nodes,
				[]*v1.Node{nodes[0]},
				[]*v1.Node{},
				nodes[1:],
				nodeGroupsState[testNodeGroup.ID()],
				0, // not used in TryRemoveTaintedNodes
			},
			false,
			0,
			false,
		},
		{
			"test one force tainted",
			scaleOpts{
				nodes,
				[]*v1.Node{},
				[]*v1.Node{nodes[0]},
				nodes[1:],
				nodeGroupsState[testNodeGroup.ID()],
				0, // not used in TryRemoveTaintedNodes
			},
			false,
			-1,
			false,
		},
		{
			"test one force tainted remaining tainted",
			scaleOpts{
				nodes,
				nodes[1:],
				[]*v1.Node{nodes[0]},
				[]*v1.Node{},
				nodeGroupsState[testNodeGroup.ID()],
				0, // not used in TryRemoveTaintedNodes
			},
			false,
			-1,
			false,
		},
		{
			"test all force tainted",
			scaleOpts{
				nodes,
				[]*v1.Node{},
				nodes,
				[]*v1.Node{},
				nodeGroupsState[testNodeGroup.ID()],
				0, // not used in TryRemoveTaintedNodes
			},
			false,
			-len(nodes),
			false,
		},
	}

	for _, tt := range forceTests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.TryRemoveForceTaintedNodes(tt.opts)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

// Test verifies that when ExcludeTaintedNodePods is enabled, a tainted node that still has running
// (non-daemonset) pods is NOT deleted via the soft path even after the
// soft_delete_grace_period has elapsed.
func TestTryRemoveTaintedNodes_WithExcludeTaintedNodePodsAndTaintedNodeHasPods(t *testing.T) {
	minNodes := 0
	maxNodes := 20
	softDeleteGracePeriod := "1m"
	hardDeleteGracePeriod := "10m"

	// Thresholds are set so that ~50% utilisation (from untainted pods only)
	// falls between TaintUpperCapacityThresholdPercent and ScaleUpThresholdPercent,
	// leaving nodesDelta=0 and routing execution to TryRemoveTaintedNodes.
	scaleUpThresholdPercent := 70
	taintUpperCapacityThresholdPercent := 40
	taintLowerCapacityThresholdPercent := 20

	nodeGroupOpts := NodeGroupOptions{
		Name:                               "default",
		CloudProviderGroupName:             "default",
		MinNodes:                           minNodes,
		MaxNodes:                           maxNodes,
		ScaleUpThresholdPercent:            scaleUpThresholdPercent,
		TaintUpperCapacityThresholdPercent: taintUpperCapacityThresholdPercent,
		TaintLowerCapacityThresholdPercent: taintLowerCapacityThresholdPercent,
		SoftDeleteGracePeriod:              softDeleteGracePeriod,
		HardDeleteGracePeriod:              hardDeleteGracePeriod,
		ExcludeTaintedNodePods:             true,
	}
	nodeGroups := []NodeGroupOptions{nodeGroupOpts}

	// Build one tainted node whose escalator taint timestamp is 5 minutes in
	// the past — beyond SoftDeleteGracePeriod (1m) but within HardDeleteGracePeriod (10m).
	taintedNode := test.BuildTestNode(test.NodeOpts{
		Name:    "tainted-node-with-pods",
		CPU:     2000,
		Mem:     8000,
		Tainted: false, // we set the taint manually below to control the timestamp
	})
	taintTimestamp := time.Now().Add(-5 * time.Minute)
	taintedNode.Spec.Taints = []v1.Taint{
		{
			Key:    k8s.ToBeRemovedByAutoscalerKey,
			Value:  strconv.FormatInt(taintTimestamp.Unix(), 10),
			Effect: v1.TaintEffectNoSchedule,
		},
	}

	// Build untainted nodes for capacity calculations.
	untaintedNodes := test.BuildTestNodes(2, test.NodeOpts{CPU: 2000, Mem: 8000})
	allNodes := append(untaintedNodes, taintedNode)

	// Place a regular (non-daemonset) running pod on the tainted node.
	// This pod should prevent soft-path deletion.
	podOnTaintedNode := test.BuildTestPod(test.PodOpts{
		Name:     "job-pod-on-tainted-node",
		CPU:      []int64{500},
		Mem:      []int64{1000},
		NodeName: taintedNode.Name,
		Running:  true,
		Phase:    v1.PodRunning,
	})

	// Put pods on untainted nodes at ~50% utilisation so the controller neither
	// scales up (< 70%) nor scales down (> 40%), landing on nodesDelta=0 and
	// exercising TryRemoveTaintedNodes via scaleNodeGroup.
	// Each untainted node: 2000m CPU. 2 nodes × 1000m = 2000m / 4000m total = 50%.
	var allPods []*v1.Pod
	allPods = append(allPods, podOnTaintedNode)
	for i, node := range untaintedNodes {
		allPods = append(allPods, test.BuildTestPod(test.PodOpts{
			Name:     fmt.Sprintf("untainted-pod-%d", i),
			CPU:      []int64{1000},
			Mem:      []int64{4000},
			NodeName: node.Name,
			Running:  true,
			Phase:    v1.PodRunning,
		}))
	}

	client, opts, err := buildTestClient(allNodes, allPods, nodeGroups, ListerOptions{})
	require.NoError(t, err)

	testCloudProvider := test.NewCloudProvider(1)
	initialNodeCount := int64(len(allNodes))
	testNodeGroup := test.NewNodeGroup(
		nodeGroupOpts.CloudProviderGroupName,
		nodeGroupOpts.Name,
		int64(minNodes),
		int64(maxNodes),
		initialNodeCount,
	)
	testCloudProvider.RegisterNodeGroup(testNodeGroup)

	nodeGroupsState := BuildNodeGroupsState(nodeGroupsStateOpts{
		nodeGroups: nodeGroups,
		client:     *client,
	})

	c := &Controller{
		Client:        client,
		Opts:          opts,
		stopChan:      nil,
		nodeGroups:    nodeGroupsState,
		cloudProvider: testCloudProvider,
	}

	// Drive the full scaleNodeGroup path so the NodeInfoMap ordering fix is
	// exercised end-to-end: scaleNodeGroup must build NodeInfoMap before the
	// ExcludeTaintedNodePods filter runs, otherwise NodeEmpty() incorrectly
	// returns true for tainted nodes that still have running pods.
	nodesDelta, err := c.scaleNodeGroup(nodeGroupOpts.Name, nodeGroupsState[nodeGroupOpts.Name])
	assert.NoError(t, err)
	assert.Equal(t, 0, nodesDelta,
		"tainted node with a running pod must not be deleted via the soft path when ExcludeTaintedNodePods is enabled")
	assert.Equal(t, initialNodeCount, testNodeGroup.TargetSize(),
		"cloud provider DeleteNodes should not have been called")
}

func TestTryRemoveTaintedNodes_WithExcludeTaintedNodePodsAndTaintedNodeHasNoPods(t *testing.T) {
	minNodes := 0
	maxNodes := 20
	softDeleteGracePeriod := "1m"
	hardDeleteGracePeriod := "10m"

	// Thresholds are set so that ~50% utilisation (from untainted pods only)
	// falls between TaintUpperCapacityThresholdPercent and ScaleUpThresholdPercent,
	// leaving nodesDelta=0 and routing execution to TryRemoveTaintedNodes.
	scaleUpThresholdPercent := 70
	taintUpperCapacityThresholdPercent := 40
	taintLowerCapacityThresholdPercent := 20

	nodeGroupOpts := NodeGroupOptions{
		Name:                               "default",
		CloudProviderGroupName:             "default",
		MinNodes:                           minNodes,
		MaxNodes:                           maxNodes,
		ScaleUpThresholdPercent:            scaleUpThresholdPercent,
		TaintUpperCapacityThresholdPercent: taintUpperCapacityThresholdPercent,
		TaintLowerCapacityThresholdPercent: taintLowerCapacityThresholdPercent,
		SoftDeleteGracePeriod:              softDeleteGracePeriod,
		HardDeleteGracePeriod:              hardDeleteGracePeriod,
		ExcludeTaintedNodePods:             true,
	}
	nodeGroups := []NodeGroupOptions{nodeGroupOpts}

	// Build one tainted node whose escalator taint timestamp is 5 minutes in
	// the past — beyond SoftDeleteGracePeriod (1m) but within HardDeleteGracePeriod (10m).
	// This node has NO pods, so it should be deleted via the soft path.
	taintedNode := test.BuildTestNode(test.NodeOpts{
		Name:    "tainted-node-no-pods",
		CPU:     2000,
		Mem:     8000,
		Tainted: false, // we set the taint manually below to control the timestamp
	})
	taintTimestamp := time.Now().Add(-5 * time.Minute)
	taintedNode.Spec.Taints = []v1.Taint{
		{
			Key:    k8s.ToBeRemovedByAutoscalerKey,
			Value:  strconv.FormatInt(taintTimestamp.Unix(), 10),
			Effect: v1.TaintEffectNoSchedule,
		},
	}

	// Build untainted nodes for capacity calculations.
	untaintedNodes := test.BuildTestNodes(2, test.NodeOpts{CPU: 2000, Mem: 8000})
	allNodes := append(untaintedNodes, taintedNode)

	// Put pods on untainted nodes at ~50% utilisation so the controller neither
	// scales up (< 70%) nor scales down (> 40%), landing on nodesDelta=0 and
	// exercising TryRemoveTaintedNodes via scaleNodeGroup.
	// Each untainted node: 2000m CPU. 2 nodes × 1000m = 2000m / 4000m total = 50%.
	// No pods on the tainted node.
	var allPods []*v1.Pod
	for i, node := range untaintedNodes {
		allPods = append(allPods, test.BuildTestPod(test.PodOpts{
			Name:     fmt.Sprintf("untainted-pod-%d", i),
			CPU:      []int64{1000},
			Mem:      []int64{4000},
			NodeName: node.Name,
			Running:  true,
			Phase:    v1.PodRunning,
		}))
	}

	client, opts, err := buildTestClient(allNodes, allPods, nodeGroups, ListerOptions{})
	require.NoError(t, err)

	testCloudProvider := test.NewCloudProvider(1)
	initialNodeCount := int64(len(allNodes))
	testNodeGroup := test.NewNodeGroup(
		nodeGroupOpts.CloudProviderGroupName,
		nodeGroupOpts.Name,
		int64(minNodes),
		int64(maxNodes),
		initialNodeCount,
	)
	testCloudProvider.RegisterNodeGroup(testNodeGroup)

	nodeGroupsState := BuildNodeGroupsState(nodeGroupsStateOpts{
		nodeGroups: nodeGroups,
		client:     *client,
	})

	c := &Controller{
		Client:        client,
		Opts:          opts,
		stopChan:      nil,
		nodeGroups:    nodeGroupsState,
		cloudProvider: testCloudProvider,
	}

	// Drive the full scaleNodeGroup path. The tainted node has no pods,
	// so it should be deleted via the soft path.
	nodesDelta, err := c.scaleNodeGroup(nodeGroupOpts.Name, nodeGroupsState[nodeGroupOpts.Name])
	assert.NoError(t, err)
	assert.Equal(t, 0, nodesDelta,
		"nodesDelta should be 0 as utilisation is within thresholds")
	assert.Equal(t, initialNodeCount-1, testNodeGroup.TargetSize(),
		"cloud provider DeleteNodes should have been called for the empty tainted node")
}
