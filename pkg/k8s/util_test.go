package k8s_test

import (
	"testing"

	"github.com/atlassian/escalator/pkg/k8s"
	"github.com/atlassian/escalator/pkg/test"
	"github.com/stretchr/testify/assert"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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

	type args struct {
		pods []*v1.Pod
	}
	tests := []struct {
		name string
		args args
		mem  resource.Quantity
		cpu  resource.Quantity
	}{
		{
			"test 1000 + 1000 == 2000",
			args{
				[]*v1.Pod{p1, p2},
			},
			*resource.NewQuantity(2000, resource.DecimalSI),
			*resource.NewMilliQuantity(2000, resource.DecimalSI),
		},
		{
			"test 1000,1000 + 300,800 == 1300,1800",
			args{
				[]*v1.Pod{p1, p3},
			},
			*resource.NewQuantity(1800, resource.DecimalSI),
			*resource.NewMilliQuantity(1300, resource.DecimalSI),
		},
		{
			"test 1000,1000 + 300,800 + 200,50000 == 1500,51800",
			args{
				[]*v1.Pod{p1, p3, p4},
			},
			*resource.NewQuantity(51800, resource.DecimalSI),
			*resource.NewMilliQuantity(1500, resource.DecimalSI),
		},
		{
			"test 1000+0 = 1000",
			args{
				[]*v1.Pod{p1, p5},
			},
			*resource.NewQuantity(1000, resource.DecimalSI),
			*resource.NewMilliQuantity(1000, resource.DecimalSI),
		},
		{
			"test 0+1000 = 1000",
			args{
				[]*v1.Pod{p5, p1},
			},
			*resource.NewQuantity(1000, resource.DecimalSI),
			*resource.NewMilliQuantity(1000, resource.DecimalSI),
		},
		{
			"test 0",
			args{
				[]*v1.Pod{p5},
			},
			*resource.NewQuantity(0, resource.DecimalSI),
			*resource.NewQuantity(0, resource.DecimalSI),
		},
		{
			"test 0+0",
			args{
				[]*v1.Pod{p5, p5},
			},
			*resource.NewQuantity(0, resource.DecimalSI),
			*resource.NewQuantity(0, resource.DecimalSI),
		},
		{
			"test multiple containers",
			args{
				[]*v1.Pod{p6, p7},
			},
			*resource.NewQuantity(2055, resource.DecimalSI),
			*resource.NewMilliQuantity(2112, resource.DecimalSI),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mem, cpu, err := k8s.CalculatePodsRequestsTotal(tt.args.pods)
			assert.Equal(t, tt.mem, mem)
			assert.Equal(t, tt.cpu, cpu)
			assert.NoError(t, err)
		})
	}
}

func TestCalculateNodesCapacityTotal(t *testing.T) {
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
		mem  resource.Quantity
		cpu  resource.Quantity
	}{
		{
			"test 1000 + 1000 == 2000",
			args{
				[]*v1.Node{n1, n2},
			},
			*resource.NewQuantity(2000, resource.DecimalSI),
			*resource.NewMilliQuantity(2000, resource.DecimalSI),
		},
		{
			"test 1000,1000 + 300,800 == 1300,1800",
			args{
				[]*v1.Node{n1, n3},
			},
			*resource.NewQuantity(1800, resource.DecimalSI),
			*resource.NewMilliQuantity(1300, resource.DecimalSI),
		},
		{
			"test 1000,1000 + 300,800 + 200,50000 == 1500,51800",
			args{
				[]*v1.Node{n1, n3, n4},
			},
			*resource.NewQuantity(51800, resource.DecimalSI),
			*resource.NewMilliQuantity(1500, resource.DecimalSI),
		},
		{
			"test 1000+0 = 1000",
			args{
				[]*v1.Node{n1, n5},
			},
			*resource.NewQuantity(1000, resource.DecimalSI),
			*resource.NewMilliQuantity(1000, resource.DecimalSI),
		},
		{
			"test 0+1000 = 1000",
			args{
				[]*v1.Node{n5, n1},
			},
			*resource.NewQuantity(1000, resource.DecimalSI),
			*resource.NewMilliQuantity(1000, resource.DecimalSI),
		},
		{
			"test 0",
			args{
				[]*v1.Node{n5},
			},
			*resource.NewQuantity(0, resource.DecimalSI),
			*resource.NewQuantity(0, resource.DecimalSI),
		},
		{
			"test 0+0",
			args{
				[]*v1.Node{n5, n5},
			},
			*resource.NewQuantity(0, resource.DecimalSI),
			*resource.NewQuantity(0, resource.DecimalSI),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mem, cpu, err := k8s.CalculateNodesCapacityTotal(tt.args.nodes)
			assert.Equal(t, tt.mem, mem)
			assert.Equal(t, tt.cpu, cpu)
			assert.NoError(t, err)
		})
	}
}
