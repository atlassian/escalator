package k8s

import (
	"testing"
	"k8s.io/api/core/v1"
	"github.com/atlassian/escalator/pkg/test"
	"github.com/stretchr/testify/assert"
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
			nodeInfos := CreateNodeNameToInfoMap(tt.args.pods, tt.args.nodes)
			assert.Len(t, nodeInfos, len(tt.args.nodes))

			var validPodCount int
			for _, pod := range tt.args.pods {
				if _, ok := nodeInfos[pod.Spec.NodeName]; ok {
					validPodCount += 1
				}
			}

			var podCount int
			for _, nodeInfo := range nodeInfos {
				podCount += len(nodeInfo.Pods())
			}
			assert.Equal(t, validPodCount, podCount)
		})
	}
}
