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

// ComputePodResourceRequest returns a framework.Resource that covers the largest
// width in each resource dimension. Because init-containers run sequentially, we collect
// the max in each dimension iteratively. In contrast, we sum the resource vectors for
// regular containers since they run simultaneously.
//
// If Pod Overhead is specified, the resources defined for Overhead
// are added to the calculated Resource request sum
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

	// take max_resource(sum_pod, any_init_container)
	for _, container := range pod.Spec.InitContainers {
		resource.SetMaxResource(container.Resources.Requests)
	}

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
