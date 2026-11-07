package scheduler

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestComputePodResourceRequestCountsRestartableInitContainers(t *testing.T) {
	always := v1.ContainerRestartPolicyAlways
	container := func(name, cpu, memory string, sidecar bool) v1.Container {
		c := v1.Container{Name: name, Resources: v1.ResourceRequirements{Requests: v1.ResourceList{
			v1.ResourceCPU: resource.MustParse(cpu), v1.ResourceMemory: resource.MustParse(memory),
		}}}
		if sidecar {
			c.RestartPolicy = &always
		}
		return c
	}
	sidecarA := container("sidecar-a", "500m", "64Mi", true)
	sidecarB := container("sidecar-b", "250m", "128Mi", true)
	setup := container("setup", "1", "256Mi", false)
	app := container("app", "2", "64Mi", false)
	tests := []struct {
		name   string
		init   []v1.Container
		cpu    int64
		memory string
	}{
		{"interleaved", []v1.Container{sidecarA, setup, sidecarB}, 2800, "336Mi"},
		{"consecutive sidecars", []v1.Container{sidecarA, sidecarB, setup}, 2800, "464Mi"},
		{"regular init first", []v1.Container{setup, sidecarA, sidecarB}, 2800, "272Mi"},
		{"no sidecars", []v1.Container{setup}, 2050, "272Mi"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &v1.Pod{Spec: v1.PodSpec{
				InitContainers: tt.init,
				Containers:     []v1.Container{app},
				Overhead:       v1.ResourceList{v1.ResourceCPU: resource.MustParse("50m"), v1.ResourceMemory: resource.MustParse("16Mi")},
			}}
			got := ComputePodResourceRequest(pod)
			if got.MilliCPU != tt.cpu {
				t.Errorf("CPU request = %dm, want %dm", got.MilliCPU, tt.cpu)
			}
			memory := resource.MustParse(tt.memory)
			if got.Memory != memory.Value() {
				t.Errorf("memory request = %d bytes, want %d bytes", got.Memory, memory.Value())
			}
		})
	}
}
