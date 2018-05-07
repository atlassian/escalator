package k8s

import (
	"github.com/atlassian/escalator/pkg/test"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
	"testing"
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
					validPodCount += 1
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
			var nodeInfo map[string]*schedulercache.NodeInfo
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
