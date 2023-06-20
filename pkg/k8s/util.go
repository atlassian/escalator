package k8s

import (
	"github.com/atlassian/escalator/pkg/k8s/scheduler"
	v1 "k8s.io/api/core/v1"
)

type PodRequestedUsage struct {
	Total         scheduler.Resource
	LargestMemory scheduler.Resource
	LargestCPU    scheduler.Resource
}

type NodeAvailableCapacity struct {
	Total                  scheduler.Resource
	LargestAvailableMemory scheduler.Resource
	LargestAvailableCPU    scheduler.Resource
}

func newPodRequestedUsage() PodRequestedUsage {
	return PodRequestedUsage{
		Total:         scheduler.NewEmptyResource(),
		LargestMemory: scheduler.NewEmptyResource(),
		LargestCPU:    scheduler.NewEmptyResource(),
	}
}

func newNodeAvailableCapacity() NodeAvailableCapacity {
	return NodeAvailableCapacity{
		Total:                  scheduler.NewEmptyResource(),
		LargestAvailableMemory: scheduler.NewEmptyResource(),
		LargestAvailableCPU:    scheduler.NewEmptyResource(),
	}
}

// PodIsDaemonSet returns if the pod is a daemonset or not
func PodIsDaemonSet(pod *v1.Pod) bool {
	for _, ownerReference := range pod.ObjectMeta.OwnerReferences {
		if ownerReference.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}

// PodIsStatic returns if the pod is static or not
func PodIsStatic(pod *v1.Pod) bool {
	configSource, ok := pod.ObjectMeta.Annotations["kubernetes.io/config.source"]
	return ok && configSource == "file"
}

// CalculatePodsRequestedUsage returns the total capacity of all pods
func CalculatePodsRequestedUsage(pods []*v1.Pod) (PodRequestedUsage, error) {
	ret := newPodRequestedUsage()

	for _, pod := range pods {
		podResources := scheduler.ComputePodResourceRequest(pod)
		ret.Total.Memory += podResources.Memory
		ret.Total.MilliCPU += podResources.MilliCPU
		if pod.Status.Phase == v1.PodPending {
			if podResources.Memory > ret.LargestMemory.Memory {
				ret.LargestMemory = scheduler.NewResource(podResources.MilliCPU, podResources.Memory)
			}
			if podResources.MilliCPU > ret.LargestCPU.MilliCPU {
				ret.LargestCPU = scheduler.NewResource(podResources.MilliCPU, podResources.Memory)
			}
		}
	}

	return ret, nil
}

// CalculateNodesCapacity calculates the total Allocatable node capacity for all nodes
func CalculateNodesCapacity(nodes []*v1.Node, pods []*v1.Pod) (NodeAvailableCapacity, error) {
	ret := newNodeAvailableCapacity()

	mappedPods := mapPodsToNode(pods)
	for _, node := range nodes {
		ret.Total.Memory += node.Status.Allocatable.Memory().Value()
		ret.Total.MilliCPU += node.Status.Allocatable.Cpu().MilliValue()
		availableResource := getNodeAvailableResources(node, mappedPods)
		if availableResource.MilliCPU > ret.LargestAvailableCPU.MilliCPU {
			ret.LargestAvailableCPU = scheduler.NewResource(
				availableResource.MilliCPU,
				availableResource.Memory,
			)
		}
		if availableResource.Memory > ret.LargestAvailableMemory.Memory {
			ret.LargestAvailableMemory = scheduler.NewResource(
				availableResource.MilliCPU,
				availableResource.Memory,
			)
		}
	}

	return ret, nil
}

// Hashes pods to their assigned nodes (via pod.Spec.NodeName), allowing quicker lookup later
func mapPodsToNode(pods []*v1.Pod) map[string]([]*v1.Pod) {
	ret := make(map[string]([]*v1.Pod))
	for _, pod := range pods {
		name := pod.Spec.NodeName
		val, found := ret[name]
		if !found {
			ret[name] = make([]*v1.Pod, 0)
			val = ret[name]
		}
		ret[name] = append(val, pod)
	}
	return ret
}

// Map each pod with f then reduce with sum. Also checks the Pod is relevant to the node.
func sumPodResourceWithFunc(pods []*v1.Pod, f func(*v1.Pod) int64) int64 {
	ret := int64(0)
	for _, pod := range pods {
		if isPodUsingNodeResources(pod) {
			ret += f(pod)
		}
	}
	return ret
}

// Determine if a pod has the PodScheduled condition as true
func isPodScheduled(pod *v1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodScheduled {
			return condition.Status == v1.ConditionTrue
		}
	}
	return false
}

// Determine if we should count a pod as using a node's resources for starvation calculations
func isPodUsingNodeResources(pod *v1.Pod) bool {
	return isPodScheduled(pod) &&
		(pod.Status.Phase == v1.PodPending || pod.Status.Phase == v1.PodRunning)
}

// Calculate how much free CPU/Memory a given node has
// Subtracts the requested pod usage of every assigned pod from the allocated resources
func getNodeAvailableResources(node *v1.Node, pods map[string]([]*v1.Pod)) scheduler.Resource {
	filteredPods := pods[node.Name]
	usedCpu := sumPodResourceWithFunc(filteredPods, func(pod *v1.Pod) int64 {
		podResources := scheduler.ComputePodResourceRequest(pod)
		return podResources.MilliCPU
	})
	usedMemory := sumPodResourceWithFunc(
		filteredPods, func(pod *v1.Pod) int64 {
			podResources := scheduler.ComputePodResourceRequest(pod)
			return podResources.Memory
		},
	)
	return scheduler.NewResource(
		node.Status.Allocatable.Cpu().MilliValue()-usedCpu,
		node.Status.Allocatable.Memory().Value()-usedMemory,
	)
}
