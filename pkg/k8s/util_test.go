package k8s_test

import (
	"testing"

	"github.com/atlassian/escalator/pkg/k8s"
	"github.com/atlassian/escalator/pkg/k8s/resource"
	"github.com/atlassian/escalator/pkg/test"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

func TestPodIsDaemonSet(t *testing.T) {
	daemon := test.BuildTestPod(test.PodOpts{
		Owner: "DaemonSet",
	})
	pod := test.BuildTestPod(test.PodOpts{})

	assert.True(t, k8s.PodIsDaemonSet(daemon))
	assert.False(t, k8s.PodIsDaemonSet(pod))
}

func TestPodIsStatic(t *testing.T) {
	staticPod := test.BuildTestPod(test.PodOpts{})
	staticPod.ObjectMeta.Annotations = make(map[string]string)
	staticPod.ObjectMeta.Annotations["kubernetes.io/config.source"] = "file"
	pod := test.BuildTestPod(test.PodOpts{})

	assert.True(t, k8s.PodIsStatic(staticPod))
	assert.False(t, k8s.PodIsStatic(pod))
}

func TestCalculatePodsRequestTotal(t *testing.T) {
	p1 := test.BuildTestPod(test.PodOpts{
		CPU: []int64{1000},
		Mem: []int64{1000},
	})
	p2 := test.BuildTestPod(test.PodOpts{
		CPU: []int64{1000},
		Mem: []int64{1000},
	})
	p3 := test.BuildTestPod(test.PodOpts{
		CPU: []int64{300},
		Mem: []int64{800},
	})
	p4 := test.BuildTestPod(test.PodOpts{
		CPU: []int64{200},
		Mem: []int64{50000},
	})
	p5 := test.BuildTestPod(test.PodOpts{
		CPU: []int64{0},
		Mem: []int64{0},
	})
	p6 := test.BuildTestPod(test.PodOpts{
		CPU: []int64{100, 200, 300},
		Mem: []int64{100, 100, 100},
	})
	p7 := test.BuildTestPod(test.PodOpts{
		CPU: []int64{22, 60, 430, 1000},
		Mem: []int64{225, 100, 430, 1000},
	})
	p8 := test.BuildTestPod(test.PodOpts{
		CPU:         []int64{22, 60, 430, 1000},
		Mem:         []int64{225, 100, 430, 1000},
		CPUOverhead: 1000,
		MemOverhead: 2000,
	})
	p9 := test.BuildTestPod(test.PodOpts{
		CPU:         []int64{100, 200, 300},
		Mem:         []int64{100, 100, 100},
		CPUOverhead: 10000,
		MemOverhead: 3000,
	})
	p10 := test.BuildTestPod(test.PodOpts{
		InitContainersCPU: []int64{20000},
		InitContainersMem: []int64{500},
		CPU:               []int64{100, 200, 300},
		Mem:               []int64{100, 100, 100},
	})
	p11 := test.BuildTestPod(test.PodOpts{
		InitContainersCPU: []int64{100},
		InitContainersMem: []int64{500},
		CPU:               []int64{100, 200, 300},
		Mem:               []int64{100, 100, 100},
	})
	p12 := test.BuildTestPod(test.PodOpts{
		InitContainersCPU: []int64{20000},
		InitContainersMem: []int64{500},
		CPU:               []int64{100, 200, 300},
		Mem:               []int64{100, 100, 100},
		CPUOverhead:       10000,
		MemOverhead:       3000,
	})

	type args struct {
		pods []*v1.Pod
	}
	tests := []struct {
		name string
		args args
		mem  int64
		cpu  int64
	}{
		{
			"test 1000 + 1000 == 2000",
			args{
				[]*v1.Pod{p1, p2},
			},
			2000,
			2000,
		},
		{
			"test 1000,1000 + 300,800 == 1300,1800",
			args{
				[]*v1.Pod{p1, p3},
			},
			1800,
			1300,
		},
		{
			"test 1000,1000 + 300,800 + 200,50000 == 1500,51800",
			args{
				[]*v1.Pod{p1, p3, p4},
			},
			51800,
			1500,
		},
		{
			"test 1000+0 = 1000",
			args{
				[]*v1.Pod{p1, p5},
			},
			1000,
			1000,
		},
		{
			"test 0+1000 = 1000",
			args{
				[]*v1.Pod{p5, p1},
			},
			1000,
			1000,
		},
		{
			"test 0",
			args{
				[]*v1.Pod{p5},
			},
			0,
			0,
		},
		{
			"test 0+0",
			args{
				[]*v1.Pod{p5, p5},
			},
			0,
			0,
		},
		{
			"test multiple containers",
			args{
				[]*v1.Pod{p6, p7},
			},
			2055,
			2112,
		},
		{
			"test pod overhead",
			args{
				[]*v1.Pod{p8},
			},
			3755,
			2512,
		},
		{
			"test pod overhead, multiple pods",
			args{
				[]*v1.Pod{p8, p9},
			},
			7055,
			13112,
		},
		{
			"test init containers",
			args{
				[]*v1.Pod{p10},
			},
			500,
			20000,
		},
		{
			"test init containers 2",
			args{
				[]*v1.Pod{p11},
			},
			500,
			600,
		},
		{
			"test init containers with pod overhead",
			args{
				[]*v1.Pod{p12},
			},
			3500,
			30000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podRequests, err := k8s.CalculatePodsRequestedUsage(tt.args.pods)
			expectedMem := *resource.NewMemoryQuantity(tt.mem)
			expectedCPU := *resource.NewCPUQuantity(tt.cpu)
			assert.Equal(t, expectedMem, *podRequests.Total.GetMemoryQuantity())
			assert.Equal(t, expectedCPU, *podRequests.Total.GetCPUQuantity())
			assert.NoError(t, err)
		})
	}
}

func TestCalculateNodesCapacity(t *testing.T) {
	n1 := test.BuildTestNode(test.NodeOpts{
		CPU: 1000,
		Mem: 1000,
	})
	n2 := test.BuildTestNode(test.NodeOpts{
		CPU: 1000,
		Mem: 1000,
	})
	n3 := test.BuildTestNode(test.NodeOpts{
		CPU: 300,
		Mem: 800,
	})
	n4 := test.BuildTestNode(test.NodeOpts{
		CPU: 200,
		Mem: 50000,
	})
	n5 := test.BuildTestNode(test.NodeOpts{
		CPU: 0,
		Mem: 0,
	})

	type args struct {
		nodes []*v1.Node
	}
	tests := []struct {
		name string
		args args
		mem  int64
		cpu  int64
	}{
		{
			"test 1000 + 1000 == 2000",
			args{
				[]*v1.Node{n1, n2},
			},
			2000,
			2000,
		},
		{
			"test 1000,1000 + 300,800 == 1300,1800",
			args{
				[]*v1.Node{n1, n3},
			},
			1800,
			1300,
		},
		{
			"test 1000,1000 + 300,800 + 200,50000 == 1500,51800",
			args{
				[]*v1.Node{n1, n3, n4},
			},
			51800,
			1500,
		},
		{
			"test 1000+0 = 1000",
			args{
				[]*v1.Node{n1, n5},
			},
			1000,
			1000,
		},
		{
			"test 0+1000 = 1000",
			args{
				[]*v1.Node{n5, n1},
			},
			1000,
			1000,
		},
		{
			"test 0",
			args{
				[]*v1.Node{n5},
			},
			0,
			0,
		},
		{
			"test 0+0",
			args{
				[]*v1.Node{n5, n5},
			},
			0,
			0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedCpu := *resource.NewCPUQuantity(tt.cpu)
			expectedMem := *resource.NewMemoryQuantity(tt.mem)
			nodeCapacity, err := k8s.CalculateNodesCapacity(tt.args.nodes, make([]*v1.Pod, 0))
			assert.Equal(t, expectedMem, *nodeCapacity.Total.GetMemoryQuantity())
			assert.Equal(t, expectedCpu, *nodeCapacity.Total.GetCPUQuantity())
			assert.NoError(t, err)
		})
	}
}
