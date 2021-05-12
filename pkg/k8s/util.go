package k8s

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

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

// CalculatePodsRequestsTotal returns the total capacity of all pods
func CalculatePodsRequestsTotal(pods []*v1.Pod) (resource.Quantity, resource.Quantity, error) {
	var memoryRequests resource.Quantity
	var cpuRequests resource.Quantity

	// todo: what about initContainers? what happens if the requests from the initContainers is larger than the containers?
	for _, pod := range pods {
		// Include the pod overhead, if configured
		memoryRequests.Add(*pod.Spec.Overhead.Memory())
		cpuRequests.Add(*pod.Spec.Overhead.Cpu())

		for _, container := range pod.Spec.Containers {
			memoryRequests.Add(*container.Resources.Requests.Memory())
			cpuRequests.Add(*container.Resources.Requests.Cpu())
		}
	}

	return memoryRequests, cpuRequests, nil
}

// CalculateNodesCapacityTotal calculates the total Allocatable node capacity for all nodes
func CalculateNodesCapacityTotal(nodes []*v1.Node) (resource.Quantity, resource.Quantity, error) {
	var memoryCapacity resource.Quantity
	var cpuCapacity resource.Quantity

	for _, node := range nodes {
		memoryCapacity.Add(*node.Status.Allocatable.Memory())
		cpuCapacity.Add(*node.Status.Allocatable.Cpu())
	}

	return memoryCapacity, cpuCapacity, nil
}
