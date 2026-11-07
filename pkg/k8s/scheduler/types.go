package scheduler

import (
	k8s_resource "github.com/atlassian/escalator/pkg/k8s/resource"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Resource is a collection of compute resource.
type Resource struct {
	MilliCPU int64
	Memory   int64
}

func NewEmptyResource() Resource {
	return Resource{
		MilliCPU: 0,
		Memory:   0,
	}
}

func NewResource(cpu int64, memory int64) Resource {
	return Resource{
		MilliCPU: cpu,
		Memory:   memory,
	}
}

// Add adds ResourceList into Resource.
func (r *Resource) Add(rl v1.ResourceList) {
	if r == nil {
		return
	}

	for rName, rQuant := range rl {
		switch rName {
		case v1.ResourceCPU:
			r.MilliCPU += rQuant.MilliValue()
		case v1.ResourceMemory:
			r.Memory += rQuant.Value()
		}
	}
}

func (r *Resource) GetCPUQuantity() *resource.Quantity {
	return k8s_resource.NewCPUQuantity(r.MilliCPU)
}

func (r *Resource) GetMemoryQuantity() *resource.Quantity {
	return k8s_resource.NewMemoryQuantity(r.Memory)
}

func (r *Resource) IsEmpty() bool {
	return r.MilliCPU == 0 && r.Memory == 0
}

// SetMaxResource compares with ResourceList and takes max value for each Resource.
func (r *Resource) SetMaxResource(rl v1.ResourceList) {
	if r == nil {
		return
	}

	for rName, rQuantity := range rl {
		switch rName {
		case v1.ResourceMemory:
			r.Memory = max(r.Memory, rQuantity.Value())
		case v1.ResourceCPU:
			r.MilliCPU = max(r.MilliCPU, rQuantity.MilliValue())
		}
	}
}

// ComputePodResourceRequest returns a Resource covering the largest width in each
// resource dimension. Regular containers run simultaneously and their requests are
// summed. Restartable init containers (sidecars) also remain running after startup, so
// their cumulative requests are included alongside regular containers and in the peak
// request for each init-container startup phase.
//
// If Pod Overhead is specified, its resources are added to the calculated request.
//
// Example:
//
// Pod:
//
//	InitContainers
//	  IC1:
//	    CPU: 2
//	    Memory: 1G
//	  IC2:
//	    CPU: 2
//	    Memory: 3G
//	Containers
//	  C1:
//	    CPU: 2
//	    Memory: 1G
//	  C2:
//	    CPU: 1
//	    Memory: 1G
//
// Result: CPU: 3, Memory: 3G
func ComputePodResourceRequest(pod *v1.Pod) *Resource {
	resource := &Resource{}
	for _, container := range pod.Spec.Containers {
		resource.Add(container.Resources.Requests)
	}

	// Init containers run sequentially, except restartable init containers (sidecars)
	// which continue running alongside later init containers and regular containers.
	restartableInitRequests := &Resource{}
	peakInitRequests := &Resource{}
	for _, container := range pod.Spec.InitContainers {
		containerRequests := &Resource{}
		containerRequests.Add(container.Resources.Requests)
		if container.RestartPolicy != nil && *container.RestartPolicy == v1.ContainerRestartPolicyAlways {
			resource.Add(container.Resources.Requests)
			restartableInitRequests.Add(container.Resources.Requests)
			containerRequests = restartableInitRequests
		} else {
			containerRequests.MilliCPU += restartableInitRequests.MilliCPU
			containerRequests.Memory += restartableInitRequests.Memory
		}

		peakInitRequests.MilliCPU = max(peakInitRequests.MilliCPU, containerRequests.MilliCPU)
		peakInitRequests.Memory = max(peakInitRequests.Memory, containerRequests.Memory)
	}

	// The pod request is the component-wise maximum of the regular-container total
	// (including sidecars) and the peak request during any init-container phase.
	resource.MilliCPU = max(resource.MilliCPU, peakInitRequests.MilliCPU)
	resource.Memory = max(resource.Memory, peakInitRequests.Memory)

	// If Overhead is being utilized, add to the total requests for the pod
	if pod.Spec.Overhead != nil {
		resource.Add(pod.Spec.Overhead)
	}

	return resource
}

func max(a, b int64) int64 {
	if a >= b {
		return a
	}
	return b
}
