package controller

import (
	"testing"
	duration "time"

	"github.com/atlassian/escalator/pkg/test"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	time "github.com/stephanos/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type ListerOptions struct {
	podListerOptions  test.PodListerOptions
	nodeListerOptions test.NodeListerOptions
}

func buildTestNodes(amount int, CPU int64, Mem int64) []*v1.Node {
	return test.BuildTestNodes(amount, test.NodeOpts{
		CPU: CPU,
		Mem: Mem,
	})
}

func buildTestPods(amount int, CPU int64, Mem int64) []*v1.Pod {
	return test.BuildTestPods(amount, test.PodOpts{
		CPU: []int64{CPU},
		Mem: []int64{Mem},
	})
}

func buildTestClient(nodes []*v1.Node, pods []*v1.Pod, nodeGroups []NodeGroupOptions, listerOptions ListerOptions) (*Client, Opts, error) {
	fakeClient, _ := test.BuildFakeClient(nodes, pods)
	opts := Opts{
		K8SClient:    fakeClient,
		NodeGroups:   nodeGroups,
		ScanInterval: 1 * duration.Minute,
		DryMode:      false,
	}
	allPodLister, err := test.NewTestPodWatcher(pods, listerOptions.podListerOptions)
	if err != nil {
		return nil, opts, err
	}

	allNodeLister, err := test.NewTestNodeWatcher(nodes, listerOptions.nodeListerOptions)
	if err != nil {
		return nil, opts, err
	}

	nodeGroupListerMap := make(map[string]*NodeGroupLister)
	for _, ng := range nodeGroups {
		if ng.Name == DefaultNodeGroup {
			nodeGroupListerMap[ng.Name] = NewDefaultNodeGroupLister(allPodLister, allNodeLister, ng)
		} else {
			nodeGroupListerMap[ng.Name] = NewNodeGroupLister(allPodLister, allNodeLister, ng)
		}
	}

	client := &Client{
		fakeClient,
		nodeGroupListerMap,
		allPodLister,
		allNodeLister,
	}

	return client, opts, nil
}

// Test the edge case where the min nodes gets changed to above the current number of untainted nodes
// the controller should untaint all tainted nodes to get above the new min ASG size instead of bringing up new nodes first
func TestUntaintNodeGroupMinNodes(t *testing.T) {
	t.Run("10 minNodes, 10 tainted, 0 untainted - scale up by untainting", func(t *testing.T) {
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

		client, opts, err := buildTestClient(nodes, buildTestPods(10, 1000, 1000), nodeGroups, ListerOptions{})
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

		controller := &Controller{
			Client:        client,
			Opts:          opts,
			stopChan:      nil,
			nodeGroups:    nodeGroupsState,
			cloudProvider: testCloudProvider,
		}

		_, err = controller.scaleNodeGroup(nodeGroup.Name, nodeGroupsState[nodeGroup.Name])
		assert.NoError(t, err)

		untainted, tainted, _ := controller.filterNodes(nodeGroupsState[nodeGroup.Name], nodes)
		// Ensure that the tainted nodes where untainted
		assert.Equal(t, minNodes, len(untainted))
		assert.Equal(t, 0, len(tainted))
	})
}

//Test if when the cluster has nodes = MaxNodes but some of these nodes are tainted
// it will untaint them before trying to scale up the cloud provider
func TestUntaintNodeGroupMaxNodes(t *testing.T) {
	t.Run("10 maxNodes, 5 tainted, 5 untainted - scale up", func(t *testing.T) {
		minNodes := 2
		maxNodes := 10
		nodeGroup := NodeGroupOptions{
			Name:                    "default",
			CloudProviderGroupName:  "default",
			MinNodes:                minNodes,
			MaxNodes:                maxNodes,
			ScaleUpThresholdPercent: 70,
		}
		nodeGroups := []NodeGroupOptions{nodeGroup}

		nodes := test.BuildTestNodes(5, test.NodeOpts{
			CPU:     1000,
			Mem:     1000,
			Tainted: true,
		})

		nodes = append(nodes, test.BuildTestNodes(5, test.NodeOpts{
			CPU: 1000,
			Mem: 1000,
		})...)

		client, opts, err := buildTestClient(nodes, buildTestPods(10, 1000, 1000), nodeGroups, ListerOptions{})
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

		controller := &Controller{
			Client:        client,
			Opts:          opts,
			stopChan:      nil,
			nodeGroups:    nodeGroupsState,
			cloudProvider: testCloudProvider,
		}

		_, err = controller.scaleNodeGroup(nodeGroup.Name, nodeGroupsState[nodeGroup.Name])
		require.NoError(t, err)

		untainted, tainted, _ := controller.filterNodes(nodeGroupsState[nodeGroup.Name], nodes)
		// Ensure that the tainted nodes where untainted
		assert.Equal(t, maxNodes, len(untainted))
		assert.Equal(t, 0, len(tainted))

	})
}

func TestScaleNodeGroup(t *testing.T) {
	type nodeArgs struct {
		initialAmount int
		cpu           int64
		mem           int64
	}

	type args struct {
		nodeArgs         nodeArgs
		pods             []*v1.Pod
		nodeGroupOptions NodeGroupOptions
		listerOptions    ListerOptions
	}

	tests := []struct {
		name string
		args args
		err  error
	}{
		{
			"100% cpu, 50% threshold",
			args{
				nodeArgs{10, 2000, 8000},
				buildTestPods(40, 500, 1000),
				NodeGroupOptions{
					Name:                    "default",
					CloudProviderGroupName:  "default",
					MinNodes:                5,
					MaxNodes:                100,
					ScaleUpThresholdPercent: 50,
				},
				ListerOptions{},
			},
			nil,
		},
		{
			"100% mem, 50% threshold",
			args{
				nodeArgs{10, 2000, 8000},
				buildTestPods(40, 100, 2000),
				NodeGroupOptions{
					Name:                    "default",
					CloudProviderGroupName:  "default",
					MinNodes:                5,
					MaxNodes:                100,
					ScaleUpThresholdPercent: 50,
				},
				ListerOptions{},
			},
			nil,
		},
		{
			"100% cpu, 70% threshold",
			args{
				nodeArgs{10, 2000, 8000},
				buildTestPods(40, 500, 1000),
				NodeGroupOptions{
					Name:                    "default",
					CloudProviderGroupName:  "default",
					MinNodes:                5,
					MaxNodes:                100,
					ScaleUpThresholdPercent: 70,
				},
				ListerOptions{},
			},
			nil,
		},
		{
			"150% cpu, 70% threshold",
			args{
				nodeArgs{10, 2000, 8000},
				buildTestPods(60, 500, 1000),
				NodeGroupOptions{
					Name:                    "default",
					CloudProviderGroupName:  "default",
					MinNodes:                5,
					MaxNodes:                100,
					ScaleUpThresholdPercent: 70,
				},
				ListerOptions{},
			},
			nil,
		},
		{
			"no nodes and no pods",
			args{
				nodeArgs{0, 0, 0},
				buildTestPods(0, 0, 0),
				NodeGroupOptions{
					Name:                    "default",
					CloudProviderGroupName:  "default",
					MinNodes:                0,
					MaxNodes:                10,
					ScaleUpThresholdPercent: 70,
				},
				ListerOptions{},
			},
			nil,
		},
		{
			"scale up from 0 node",
			args{
				nodeArgs{0, 1000, 10000},
				buildTestPods(1, 500, 1000),
				NodeGroupOptions{
					Name:                    "default",
					CloudProviderGroupName:  "default",
					MinNodes:                0,
					MaxNodes:                10,
					ScaleUpThresholdPercent: 70,
				},
				ListerOptions{},
			},
			nil,
		},
		{
			"node count less than the minimum",
			args{
				nodeArgs{1, 0, 0},
				buildTestPods(0, 0, 0),
				NodeGroupOptions{
					Name:                   "default",
					CloudProviderGroupName: "default",
					MinNodes:               5,
				},
				ListerOptions{},
			},
			errors.New("node count less than the minimum"),
		},
		{
			"node count larger than the maximum",
			args{
				nodeArgs{10, 0, 0},
				buildTestPods(0, 0, 0),
				NodeGroupOptions{
					Name:                   "default",
					CloudProviderGroupName: "default",
					MaxNodes:               5,
				},
				ListerOptions{},
			},
			errors.New("node count larger than the maximum"),
		},
		{
			"node and pod usage/requests",
			args{
				nodeArgs{10, 0, 0},
				buildTestPods(5, 0, 0),
				NodeGroupOptions{
					Name:                   "default",
					CloudProviderGroupName: "default",
					MinNodes:               1,
					MaxNodes:               100,
				},
				ListerOptions{},
			},
			errors.New("cannot divide by zero in percent calculation"),
		},
		{
			"invalid node and pod usage/requests",
			args{
				nodeArgs{10, -100, 0},
				buildTestPods(5, 0, -100),
				NodeGroupOptions{
					Name:                   "default",
					CloudProviderGroupName: "default",
					MinNodes:               1,
					MaxNodes:               100,
				},
				ListerOptions{},
			},
			errors.New("cannot divide by zero in percent calculation"),
		},
		{
			"invalid node and pod usage/requests",
			args{
				nodeArgs{10, -100, -100},
				buildTestPods(5, -100, -100),
				NodeGroupOptions{
					Name:                   "default",
					CloudProviderGroupName: "default",
					MinNodes:               1,
					MaxNodes:               100,
				},
				ListerOptions{},
			},
			errors.New("cannot divide by zero in percent calculation"),
		},
		{
			"lister not being able to list pods",
			args{
				nodeArgs{10, 2000, 8000},
				buildTestPods(5, 1000, 2000),
				NodeGroupOptions{
					Name:                    "default",
					CloudProviderGroupName:  "default",
					MinNodes:                1,
					MaxNodes:                100,
					ScaleUpThresholdPercent: 70,
				},
				ListerOptions{
					podListerOptions: test.PodListerOptions{
						ReturnErrorOnList: true,
					},
				},
			},
			errors.New("unable to list pods"),
		},
		{
			"lister not being able to list nodes",
			args{
				nodeArgs{10, 2000, 8000},
				buildTestPods(5, 1000, 2000),
				NodeGroupOptions{
					Name:                    "default",
					CloudProviderGroupName:  "default",
					MinNodes:                1,
					MaxNodes:                100,
					ScaleUpThresholdPercent: 70,
				},
				ListerOptions{
					nodeListerOptions: test.NodeListerOptions{
						ReturnErrorOnList: true,
					},
				},
			},
			errors.New("unable to list nodes"),
		},
		{
			"no need to scale up",
			args{
				nodeArgs{10, 2000, 8000},
				buildTestPods(5, 1000, 2000),
				NodeGroupOptions{
					Name:                    "default",
					CloudProviderGroupName:  "default",
					MinNodes:                1,
					MaxNodes:                100,
					ScaleUpThresholdPercent: 70,
				},
				ListerOptions{},
			},
			nil,
		},
		{
			"scale up test",
			args{
				nodeArgs{10, 1500, 5000},
				buildTestPods(100, 500, 600),
				NodeGroupOptions{
					Name:                    "default",
					CloudProviderGroupName:  "default",
					MinNodes:                5,
					MaxNodes:                100,
					ScaleUpThresholdPercent: 70,
				},
				ListerOptions{},
			},
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeGroups := []NodeGroupOptions{tt.args.nodeGroupOptions}
			ngName := tt.args.nodeGroupOptions.Name
			nodes := buildTestNodes(tt.args.nodeArgs.initialAmount, tt.args.nodeArgs.cpu, tt.args.nodeArgs.mem)
			client, opts, err := buildTestClient(nodes, tt.args.pods, nodeGroups, tt.args.listerOptions)
			require.NoError(t, err)

			// For these test cases we only use 1 node group/cloud provider node group
			nodeGroupSize := 1

			// Create a test (mock) cloud provider
			testCloudProvider := test.NewCloudProvider(nodeGroupSize)
			testNodeGroup := test.NewNodeGroup(
				tt.args.nodeGroupOptions.CloudProviderGroupName,
				tt.args.nodeGroupOptions.Name,
				int64(tt.args.nodeGroupOptions.MinNodes),
				int64(tt.args.nodeGroupOptions.MaxNodes),
				int64(len(nodes)),
			)
			testCloudProvider.RegisterNodeGroup(testNodeGroup)

			// Create a node group state with the mapping of node groups to the cloud providers node groups
			nodeGroupsState := BuildNodeGroupsState(nodeGroupsStateOpts{
				nodeGroups: nodeGroups,
				client:     *client,
			})

			controller := &Controller{
				Client:        client,
				Opts:          opts,
				stopChan:      nil,
				nodeGroups:    nodeGroupsState,
				cloudProvider: testCloudProvider,
			}

			nodesDelta, err := controller.scaleNodeGroup(ngName, nodeGroupsState[ngName])

			// Ensure there were no errors
			if tt.err == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, tt.err, err.Error())
			}

			if nodesDelta <= 0 {
				return
			}

			// Ensure the node group on the cloud provider side scales up to the correct amount
			assert.Equal(t, int64(len(nodes)+nodesDelta), testNodeGroup.TargetSize())

			// Create the nodes to simulate the cloud provider bringing up the new nodes
			newNodes := append(nodes, buildTestNodes(nodesDelta, tt.args.nodeArgs.cpu, tt.args.nodeArgs.mem)...)
			// Create a new client with the new nodes and update everything that uses the client
			client, opts, err = buildTestClient(newNodes, tt.args.pods, nodeGroups, tt.args.listerOptions)
			require.NoError(t, err)

			controller.Client = client
			controller.Opts = opts
			nodeGroupsState[ngName].NodeGroupLister = client.Listers[ngName]

			// Re-run the scale, ensure the result is 0 as we shouldn't need to scale up again
			newNodesDelta, _ := controller.scaleNodeGroup(ngName, nodeGroupsState[ngName])
			assert.Equal(t, 0, newNodesDelta)

		})
	}

}

func TestScaleNodeGroup_MultipleRuns(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	type args struct {
		nodes            []*v1.Node
		pods             []*v1.Pod
		nodeGroupOptions NodeGroupOptions
		listerOptions    ListerOptions
	}
	var defaultNodeCPUCapaity int64 = 2000
	var defaultNodeMemCapacity int64 = 8000

	tests := []struct {
		name                      string
		args                      args
		scaleUpWithCachedCapacity bool
		runs                      int
		runInterval               duration.Duration
		want                      int
		err                       error
	}{
		{
			"10 nodes, 0 pods, min nodes 5, fast node removal",
			args{
				buildTestNodes(10, defaultNodeCPUCapaity, defaultNodeMemCapacity),
				buildTestPods(0, 0, 0),
				NodeGroupOptions{
					Name:                               "default",
					CloudProviderGroupName:             "default",
					MinNodes:                           5,
					MaxNodes:                           100,
					ScaleUpThresholdPercent:            70,
					TaintLowerCapacityThresholdPercent: 40,
					TaintUpperCapacityThresholdPercent: 60,
					FastNodeRemovalRate:                4,
					SlowNodeRemovalRate:                2,
					SoftDeleteGracePeriod:              "1m",
					TaintEffect:                        "NoExecute",
				},
				ListerOptions{},
			},
			false,
			1,
			duration.Minute,
			-4,
			nil,
		},
		{
			"10 nodes, 10 pods, slow node removal",
			args{
				buildTestNodes(10, defaultNodeCPUCapaity, defaultNodeMemCapacity),
				buildTestPods(10, 1000, 1000),
				NodeGroupOptions{
					Name:                               "default",
					CloudProviderGroupName:             "default",
					MinNodes:                           5,
					MaxNodes:                           100,
					ScaleUpThresholdPercent:            70,
					TaintLowerCapacityThresholdPercent: 40,
					TaintUpperCapacityThresholdPercent: 60,
					FastNodeRemovalRate:                4,
					SlowNodeRemovalRate:                2,
					SoftDeleteGracePeriod:              "5m",
					TaintEffect:                        "NoSchedule",
				},
				ListerOptions{},
			},
			false,
			5,
			duration.Minute,
			-2,
			nil,
		},
		{
			"4 nodes, 0 pods, min nodes 0, fast node removal to scale down to 0",
			args{
				buildTestNodes(4, defaultNodeCPUCapaity, defaultNodeMemCapacity),
				buildTestPods(0, 0, 0),
				NodeGroupOptions{
					Name:                               "default",
					CloudProviderGroupName:             "default",
					MinNodes:                           0,
					MaxNodes:                           100,
					ScaleUpThresholdPercent:            70,
					TaintLowerCapacityThresholdPercent: 40,
					TaintUpperCapacityThresholdPercent: 60,
					FastNodeRemovalRate:                4,
					SlowNodeRemovalRate:                2,
					SoftDeleteGracePeriod:              "1m",
					TaintEffect:                        "NoExecute",
				},
				ListerOptions{},
			},
			false,
			1,
			duration.Minute,
			-4,
			nil,
		},
		{
			"0 nodes, 10 pods, min nodes 0, scale up from 0 without cache",
			args{
				buildTestNodes(0, defaultNodeCPUCapaity, defaultNodeMemCapacity),
				buildTestPods(40, 200, 800),
				NodeGroupOptions{
					Name:                               "default",
					CloudProviderGroupName:             "default",
					MinNodes:                           0,
					MaxNodes:                           100,
					ScaleUpThresholdPercent:            70,
					TaintLowerCapacityThresholdPercent: 40,
					TaintUpperCapacityThresholdPercent: 60,
					FastNodeRemovalRate:                4,
					SlowNodeRemovalRate:                2,
					SoftDeleteGracePeriod:              "1m",
					ScaleUpCoolDownPeriod:              "1m",
					TaintEffect:                        "NoExecute",
				},
				ListerOptions{},
			},
			false,
			1,
			duration.Minute,
			1,
			nil,
		},
		{
			"0 nodes, 10 pods, min nodes 0, scale up from 0 with cache",
			args{
				buildTestNodes(0, defaultNodeCPUCapaity, defaultNodeMemCapacity),
				buildTestPods(40, 200, 800),
				NodeGroupOptions{
					Name:                               "default",
					CloudProviderGroupName:             "default",
					MinNodes:                           0,
					MaxNodes:                           100,
					ScaleUpThresholdPercent:            70,
					TaintLowerCapacityThresholdPercent: 40,
					TaintUpperCapacityThresholdPercent: 60,
					FastNodeRemovalRate:                4,
					SlowNodeRemovalRate:                2,
					SoftDeleteGracePeriod:              "1m",
					ScaleUpCoolDownPeriod:              "1m",
					TaintEffect:                        "NoExecute",
				},
				ListerOptions{},
			},
			true,
			1,
			duration.Minute,
			6,
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeGroups := []NodeGroupOptions{tt.args.nodeGroupOptions}
			client, opts, err := buildTestClient(tt.args.nodes, tt.args.pods, nodeGroups, tt.args.listerOptions)
			require.NoError(t, err)

			// For these test cases we only use 1 node group/cloud provider node group
			nodeGroupSize := 1

			// Create a test (mock) cloud provider
			testCloudProvider := test.NewCloudProvider(nodeGroupSize)
			testNodeGroup := test.NewNodeGroup(
				tt.args.nodeGroupOptions.CloudProviderGroupName,
				tt.args.nodeGroupOptions.Name,
				int64(tt.args.nodeGroupOptions.MinNodes),
				int64(tt.args.nodeGroupOptions.MaxNodes),
				int64(len(tt.args.nodes)),
			)
			testCloudProvider.RegisterNodeGroup(testNodeGroup)

			// Create a node group state with the mapping of node groups to the cloud providers node groups
			nodeGroupsState := BuildNodeGroupsState(nodeGroupsStateOpts{
				nodeGroups: nodeGroups,
				client:     *client,
			})

			// add cached node allocatable capacity when configured
			if tt.scaleUpWithCachedCapacity {
				defaultNodeGroupState := nodeGroupsState[tt.args.nodeGroupOptions.Name]
				defaultNodeGroupState.cpuCapacity = *resource.NewMilliQuantity(defaultNodeCPUCapaity, resource.DecimalSI)
				defaultNodeGroupState.memCapacity = *resource.NewQuantity(defaultNodeMemCapacity, resource.DecimalSI)
				nodeGroupsState[tt.args.nodeGroupOptions.Name] = defaultNodeGroupState
			}

			controller := &Controller{
				Client:        client,
				Opts:          opts,
				stopChan:      nil,
				nodeGroups:    nodeGroupsState,
				cloudProvider: testCloudProvider,
			}

			// Create a new mock clock
			mockClock := time.NewMock()
			time.Work = mockClock

			// Run the initial run of the scale
			nodesDelta, err := controller.scaleNodeGroup(tt.args.nodeGroupOptions.Name, nodeGroupsState[tt.args.nodeGroupOptions.Name])

			// Ensure the returned nodes delta is what we wanted
			assert.Equal(t, tt.want, nodesDelta)
			assert.Equal(t, tt.err, err)

			// Run subsequent runs of the scale to "simulate" the deletion of the tainted nodes when scaling down
			for i := 0; i < tt.runs; i++ {
				mockClock.Add(tt.runInterval)
				_, err := controller.scaleNodeGroup(tt.args.nodeGroupOptions.Name, nodeGroupsState[tt.args.nodeGroupOptions.Name])
				assert.Nil(t, err)
			}

			cloudProviderNodeGroup, ok := testCloudProvider.GetNodeGroup(tt.args.nodeGroupOptions.CloudProviderGroupName)
			assert.True(t, ok)

			// Ensure the node group on the cloud provider side scales up/down to the correct amount
			assert.Equal(t, int64(len(tt.args.nodes)+nodesDelta), cloudProviderNodeGroup.TargetSize())
			assert.Equal(t, int64(len(tt.args.nodes)+nodesDelta), cloudProviderNodeGroup.Size())
		})
	}
}
