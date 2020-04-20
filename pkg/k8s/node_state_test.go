package k8s

import (
	"testing"

	"github.com/atlassian/escalator/pkg/test"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

func TestCreateNodeNameToInfoMap(t *testing.T) {
	type args struct {
		nodes []*v1.Node
		pods  []*v1.Pod
	}

	tests := []struct {
		name string
		args args
	}{
		{
			"basic test",
			args{
				[]*v1.Node{
					test.BuildTestNode(test.NodeOpts{Name: "node-1"}),
					test.BuildTestNode(test.NodeOpts{Name: "node-2"}),
					test.BuildTestNode(test.NodeOpts{Name: "node-3"}),
					test.BuildTestNode(test.NodeOpts{Name: "node-4"}),
					test.BuildTestNode(test.NodeOpts{Name: "node-5"}),
				},
				[]*v1.Pod{
					test.BuildTestPod(test.PodOpts{NodeName: "node-1"}),
					test.BuildTestPod(test.PodOpts{NodeName: "node-1"}),
					test.BuildTestPod(test.PodOpts{NodeName: "node-2"}),
				},
			},
		},
		{
			"test pods out of sync with nodes",
			args{
				[]*v1.Node{
					test.BuildTestNode(test.NodeOpts{Name: "node-1"}),
				},
				[]*v1.Pod{
					test.BuildTestPod(test.PodOpts{NodeName: "node-1"}),
					test.BuildTestPod(test.PodOpts{NodeName: "node-2"}),
					test.BuildTestPod(test.PodOpts{NodeName: "node-2"}),
					test.BuildTestPod(test.PodOpts{NodeName: "node-2"}),
					test.BuildTestPod(test.PodOpts{NodeName: "node-2"}),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeInfo := CreateNodeNameToInfoMap(tt.args.pods, tt.args.nodes)
			assert.Len(t, nodeInfo, len(tt.args.nodes))

			var validPodCount int
			for _, pod := range tt.args.pods {
				if _, ok := nodeInfo[pod.Spec.NodeName]; ok {
					validPodCount++
				}
			}

			var podCount int
			for _, ni := range nodeInfo {
				podCount += len(ni.Pods())
			}
			assert.Equal(t, validPodCount, podCount)
		})
	}
}

func TestNodeEmpty(t *testing.T) {
	type args struct {
		nodes         []*v1.Node
		pods          []*v1.Pod
		targetNode    string
		emptyNodeInfo bool
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"empty node",
			args{
				[]*v1.Node{
					test.BuildTestNode(test.NodeOpts{Name: "node-1"}),
				},
				[]*v1.Pod{},
				"node-1",
				false,
			},
			true,
		},
		{
			"node missing from nodeinfo map",
			args{
				[]*v1.Node{
					test.BuildTestNode(test.NodeOpts{Name: "node-1"}),
				},
				[]*v1.Pod{},
				"node-1",
				true,
			},
			false,
		},
		{
			"node with pods",
			args{
				[]*v1.Node{
					test.BuildTestNode(test.NodeOpts{Name: "node-1"}),
				},
				[]*v1.Pod{
					test.BuildTestPod(test.PodOpts{NodeName: "node-1"}),
					test.BuildTestPod(test.PodOpts{NodeName: "node-1"}),
				},
				"node-1",
				false,
			},
			false,
		},
		{
			"node with just daemon sets",
			args{
				[]*v1.Node{
					test.BuildTestNode(test.NodeOpts{Name: "node-1"}),
				},
				[]*v1.Pod{
					test.BuildTestPod(test.PodOpts{NodeName: "node-1", Owner: "DaemonSet"}),
					test.BuildTestPod(test.PodOpts{NodeName: "node-1", Owner: "DaemonSet"}),
				},
				"node-1",
				false,
			},
			true,
		},
		{
			"node with daemon sets and pods",
			args{
				[]*v1.Node{
					test.BuildTestNode(test.NodeOpts{Name: "node-1"}),
				},
				[]*v1.Pod{
					test.BuildTestPod(test.PodOpts{NodeName: "node-1", Owner: "DaemonSet"}),
					test.BuildTestPod(test.PodOpts{NodeName: "node-1"}),
				},
				"node-1",
				false,
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var nodeInfo map[string]*NodeInfo
			if !tt.args.emptyNodeInfo {
				nodeInfo = CreateNodeNameToInfoMap(tt.args.pods, tt.args.nodes)
			}

			var targetNode *v1.Node
			for _, node := range tt.args.nodes {
				if node.Name == tt.args.targetNode {
					targetNode = node
					break
				}
			}

			nodeEmpty := NodeEmpty(targetNode, nodeInfo)
			assert.Equal(t, tt.want, nodeEmpty)
		})
	}

}

func TestNodePodsRemaining(t *testing.T) {
	type args struct {
		nodes         []*v1.Node
		pods          []*v1.Pod
		targetNode    string
		emptyNodeInfo bool
	}

	tests := []struct {
		name      string
		args      args
		wantCount int
		wantOk    bool
	}{
		{
			"empty node",
			args{
				[]*v1.Node{
					test.BuildTestNode(test.NodeOpts{Name: "node-1"}),
				},
				[]*v1.Pod{},
				"node-1",
				false,
			},
			0,
			true,
		},
		{
			"node missing from nodeinfo map",
			args{
				[]*v1.Node{
					test.BuildTestNode(test.NodeOpts{Name: "node-1"}),
				},
				[]*v1.Pod{},
				"node-1",
				true,
			},
			0,
			false,
		},
		{
			"node with pods",
			args{
				[]*v1.Node{
					test.BuildTestNode(test.NodeOpts{Name: "node-1"}),
				},
				[]*v1.Pod{
					test.BuildTestPod(test.PodOpts{NodeName: "node-1"}),
					test.BuildTestPod(test.PodOpts{NodeName: "node-1"}),
				},
				"node-1",
				false,
			},
			2,
			true,
		},
		{
			"node with just daemon sets",
			args{
				[]*v1.Node{
					test.BuildTestNode(test.NodeOpts{Name: "node-1"}),
				},
				[]*v1.Pod{
					test.BuildTestPod(test.PodOpts{NodeName: "node-1", Owner: "DaemonSet"}),
					test.BuildTestPod(test.PodOpts{NodeName: "node-1", Owner: "DaemonSet"}),
				},
				"node-1",
				false,
			},
			0,
			true,
		},
		{
			"node with daemon sets and pods",
			args{
				[]*v1.Node{
					test.BuildTestNode(test.NodeOpts{Name: "node-1"}),
				},
				[]*v1.Pod{
					test.BuildTestPod(test.PodOpts{NodeName: "node-1", Owner: "DaemonSet"}),
					test.BuildTestPod(test.PodOpts{NodeName: "node-1"}),
				},
				"node-1",
				false,
			},
			1,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var nodeInfo map[string]*NodeInfo
			if !tt.args.emptyNodeInfo {
				nodeInfo = CreateNodeNameToInfoMap(tt.args.pods, tt.args.nodes)
			}

			var targetNode *v1.Node
			for _, node := range tt.args.nodes {
				if node.Name == tt.args.targetNode {
					targetNode = node
					break
				}
			}

			nodePodsRemaining, ok := NodePodsRemaining(targetNode, nodeInfo)
			assert.Equal(t, tt.wantCount, nodePodsRemaining)
			assert.Equal(t, tt.wantOk, ok)
		})
	}

}
