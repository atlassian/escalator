package controller

import (
	"errors"
	"testing"
	time "github.com/stephanos/clock"
	duration "time"

	"github.com/atlassian/escalator/pkg/test"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	"github.com/atlassian/escalator/pkg/cloudprovider"
	log "github.com/sirupsen/logrus"
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

func buildTestClient(nodes []*v1.Node, pods []*v1.Pod, nodeGroups []NodeGroupOptions, listerOptions ListerOptions) (*Client, Opts) {
	fakeClient, _ := test.BuildFakeClient(nodes, pods)
	opts := Opts{
		K8SClient:    fakeClient,
		NodeGroups:   nodeGroups,
		ScanInterval: 1 * duration.Minute,
		DryMode:      false,
	}
	allPodLister := test.NewTestPodWatcher(pods, listerOptions.podListerOptions)
	allNodeLister := test.NewTestNodeWatcher(nodes, listerOptions.nodeListerOptions)

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

	return client, opts
}

func TestScaleNodeGroup(t *testing.T) {
	type args struct {
		nodes            []*v1.Node
		pods             []*v1.Pod
		nodeGroupOptions NodeGroupOptions
		listerOptions    ListerOptions
	}

	tests := []struct {
		name string
		args args
		want int
		err  error
	}{
		{
			"100% cpu, 50% threshold",
			args{
				buildTestNodes(10, 2000, 8000),
				buildTestPods(40, 500, 1000),
				NodeGroupOptions{
					Name:                     "default",
					MinNodes:                 5,
					MaxNodes:                 100,
					ScaleUpThreshholdPercent: 50,
				},
				ListerOptions{},
			},
			5,
			nil,
		},
		{
			"100% mem, 50% threshold",
			args{
				buildTestNodes(10, 2000, 8000),
				buildTestPods(40, 100, 2000),
				NodeGroupOptions{
					Name:                     "default",
					MinNodes:                 5,
					MaxNodes:                 100,
					ScaleUpThreshholdPercent: 50,
				},
				ListerOptions{},
			},
			5,
			nil,
		},
		{
			"100% cpu, 70% threshold",
			args{
				buildTestNodes(10, 2000, 8000),
				buildTestPods(40, 500, 1000),
				NodeGroupOptions{
					Name:                     "default",
					MinNodes:                 5,
					MaxNodes:                 100,
					ScaleUpThreshholdPercent: 70,
				},
				ListerOptions{},
			},
			3,
			nil,
		},
		{
			"150% cpu, 70% threshold",
			args{
				buildTestNodes(10, 2000, 8000),
				buildTestPods(60, 500, 1000),
				NodeGroupOptions{
					Name:                     "default",
					MinNodes:                 5,
					MaxNodes:                 100,
					ScaleUpThreshholdPercent: 70,
				},
				ListerOptions{},
			},
			8,
			nil,
		},
		{
			"no nodes",
			args{
				buildTestNodes(0, 0, 0),
				buildTestPods(0, 0, 0),
				NodeGroupOptions{},
				ListerOptions{},
			},
			0,
			errors.New("no nodes remaining"),
		},
		{
			"node count less than the minimum",
			args{
				buildTestNodes(1, 0, 0),
				buildTestPods(0, 0, 0),
				NodeGroupOptions{
					Name:     "default",
					MinNodes: 5,
				},
				ListerOptions{},
			},
			0,
			errors.New("node count less than the minimum"),
		},
		{
			"node count larger than the maximum",
			args{
				buildTestNodes(10, 0, 0),
				buildTestPods(0, 0, 0),
				NodeGroupOptions{
					Name:     "default",
					MaxNodes: 5,
				},
				ListerOptions{},
			},
			0,
			errors.New("node count larger than the maximum"),
		},
		{
			"invalid node and pod usage/requests",
			args{
				buildTestNodes(10, 0, 0),
				buildTestPods(5, 0, 0),
				NodeGroupOptions{
					Name:     "default",
					MinNodes: 1,
					MaxNodes: 100,
				},
				ListerOptions{},
			},
			0,
			errors.New("Cannot divide by zero in percent calculation"),
		},
		{
			"invalid node and pod usage/requests",
			args{
				buildTestNodes(10, -100, 0),
				buildTestPods(5, 0, -100),
				NodeGroupOptions{
					Name:     "default",
					MinNodes: 1,
					MaxNodes: 100,
				},
				ListerOptions{},
			},
			0,
			errors.New("Cannot divide by zero in percent calculation"),
		},
		{
			"invalid node and pod usage/requests",
			args{
				buildTestNodes(10, -100, -100),
				buildTestPods(5, -100, -100),
				NodeGroupOptions{
					Name:     "default",
					MinNodes: 1,
					MaxNodes: 100,
				},
				ListerOptions{},
			},
			0,
			errors.New("Cannot divide by zero in percent calculation"),
		},
		{
			"lister not being able to list pods",
			args{
				buildTestNodes(10, 2000, 8000),
				buildTestPods(5, 1000, 2000),
				NodeGroupOptions{
					Name:                     "default",
					MinNodes:                 1,
					MaxNodes:                 100,
					ScaleUpThreshholdPercent: 70,
				},
				ListerOptions{
					podListerOptions: test.PodListerOptions{
						ReturnErrorOnList: true,
					},
				},
			},
			0,
			errors.New("unable to list pods"),
		},
		{
			"lister not being able to list nodes",
			args{
				buildTestNodes(10, 2000, 8000),
				buildTestPods(5, 1000, 2000),
				NodeGroupOptions{
					Name:                     "default",
					MinNodes:                 1,
					MaxNodes:                 100,
					ScaleUpThreshholdPercent: 70,
				},
				ListerOptions{
					nodeListerOptions: test.NodeListerOptions{
						ReturnErrorOnList: true,
					},
				},
			},
			0,
			errors.New("unable to list nodes"),
		},
		{
			"no need to scale up",
			args{
				buildTestNodes(10, 2000, 8000),
				buildTestPods(5, 1000, 2000),
				NodeGroupOptions{
					Name:                     "default",
					MinNodes:                 1,
					MaxNodes:                 100,
					ScaleUpThreshholdPercent: 70,
				},
				ListerOptions{},
			},
			0,
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeGroups := []NodeGroupOptions{tt.args.nodeGroupOptions}
			client, opts := buildTestClient(tt.args.nodes, tt.args.pods, nodeGroups, tt.args.listerOptions)

			// For these test cases we only use 1 node group/ASG
			nodeGroupSize := 1

			// Create a test (mock) cloud provider
			testCloudProvider := test.NewCloudProvider(nodeGroupSize)
			testNodeGroup := test.NewNodeGroup(
				tt.args.nodeGroupOptions.Name,
				int64(tt.args.nodeGroupOptions.MinNodes),
				int64(tt.args.nodeGroupOptions.MaxNodes),
				int64(len(tt.args.nodes)),
				false,
			)
			testCloudProvider.RegisterNodeGroup(testNodeGroup)

			// Create a node group state with the mapping of node groups to the cloud providers node groups
			asgMap := make(map[string]cloudprovider.NodeGroup, nodeGroupSize)
			asgMap[tt.args.nodeGroupOptions.Name] = testNodeGroup
			nodeGroupsState := BuildNodeGroupsState(nodeGroupsStateOpts{
				nodeGroups: nodeGroups,
				client:     *client,
				asg:        asgMap,
			})

			controller := &Controller{
				Client:        client,
				Opts:          opts,
				stopChan:      nil,
				nodeGroups:    nodeGroupsState,
				cloudProvider: testCloudProvider,
			}

			nodesDelta, err := controller.scaleNodeGroup(tt.args.nodeGroupOptions.Name, nodeGroupsState[tt.args.nodeGroupOptions.Name])

			// Ensure the returned nodes delta is what we wanted
			assert.Equal(t, tt.want, nodesDelta)
			assert.Equal(t, tt.err, err)

			// Ensure the node group on the cloud provider side scales up to the correct amount
			assert.Equal(t, int64(len(tt.args.nodes)+nodesDelta), testNodeGroup.TargetSize())
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
		scaleFailure     bool
	}

	tests := []struct {
		name        string
		args        args
		runs        int
		runInterval duration.Duration
		want        int
		err         error
	}{
		{
			"10 nodes, 0 pods, fast node removal",
			args{
				buildTestNodes(10, 2000, 8000),
				buildTestPods(0, 0, 0),
				NodeGroupOptions{
					Name:                                "default",
					MinNodes:                            5,
					MaxNodes:                            100,
					ScaleUpThreshholdPercent:            70,
					TaintLowerCapacityThreshholdPercent: 40,
					TaintUpperCapacityThreshholdPercent: 60,
					FastNodeRemovalRate:                 4,
					SlowNodeRemovalRate:                 2,
					SoftDeleteGracePeriod:				 "1m",
				},
				ListerOptions{},
				false,
			},
			1,
			duration.Minute,
			-4,
			nil,
		},
		{
			"10 nodes, 10 pods, slow node removal",
			args{
				buildTestNodes(10, 2000, 8000),
				buildTestPods(10, 1000, 1000),
				NodeGroupOptions{
					Name:                                "default",
					MinNodes:                            5,
					MaxNodes:                            100,
					ScaleUpThreshholdPercent:            70,
					TaintLowerCapacityThreshholdPercent: 40,
					TaintUpperCapacityThreshholdPercent: 60,
					FastNodeRemovalRate:                 4,
					SlowNodeRemovalRate:                 2,
					SoftDeleteGracePeriod:               "5m",
				},
				ListerOptions{},
				false,
			},
			5,
			duration.Minute,
			-2,
			nil,
		},
		{
			"test nodes failing to come up, lock timeout",
			args{
				buildTestNodes(10, 2000, 8000),
				buildTestPods(40, 500, 1000),
				NodeGroupOptions{
					Name:                     "default",
					MinNodes:                 5,
					MaxNodes:                 100,
					ScaleUpThreshholdPercent: 50,
					ScaleUpCoolDownPeriod:    "5m",
					ScaleUpCoolDownTimeout:   "10m",
				},
				ListerOptions{},
				true,
			},
			10,
			duration.Minute,
			5,
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeGroups := []NodeGroupOptions{tt.args.nodeGroupOptions}
			client, opts := buildTestClient(tt.args.nodes, tt.args.pods, nodeGroups, tt.args.listerOptions)

			// For these test cases we only use 1 node group/ASG
			nodeGroupSize := 1

			// Create a test (mock) cloud provider
			testCloudProvider := test.NewCloudProvider(nodeGroupSize)
			testNodeGroup := test.NewNodeGroup(
				tt.args.nodeGroupOptions.Name,
				int64(tt.args.nodeGroupOptions.MinNodes),
				int64(tt.args.nodeGroupOptions.MaxNodes),
				int64(len(tt.args.nodes)),
				tt.args.scaleFailure,
			)
			testCloudProvider.RegisterNodeGroup(testNodeGroup)

			// Create a node group state with the mapping of node groups to the cloud providers node groups
			asgMap := make(map[string]cloudprovider.NodeGroup, nodeGroupSize)
			asgMap[tt.args.nodeGroupOptions.Name] = testNodeGroup
			nodeGroupsState := BuildNodeGroupsState(nodeGroupsStateOpts{
				nodeGroups: nodeGroups,
				client:     *client,
				asg:        asgMap,
			})

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
			log.Debug("------- controller.scaleNodeGroup -------")
			nodesDelta, err := controller.scaleNodeGroup(tt.args.nodeGroupOptions.Name, nodeGroupsState[tt.args.nodeGroupOptions.Name])

			// Ensure the returned nodes delta is what we wanted
			assert.Equal(t, tt.want, nodesDelta)
			assert.Equal(t, tt.err, err)

			// Run subsequent runs of the scale to "simulate" the deletion of the tainted nodes
			for i := 0; i < tt.runs; i++ {
				log.Debug("------- controller.scaleNodeGroup -------")
				mockClock.Add(tt.runInterval)
				controller.scaleNodeGroup(tt.args.nodeGroupOptions.Name, nodeGroupsState[tt.args.nodeGroupOptions.Name])
			}

			// Ensure the node group on the cloud provider side scales up/down to the correct amount
			assert.Equal(t, int64(len(tt.args.nodes)+nodesDelta), testNodeGroup.TargetSize())
			if tt.args.scaleFailure {
				assert.Equal(t, int64(len(tt.args.nodes)), testNodeGroup.Size())
			}
		})
	}
}
