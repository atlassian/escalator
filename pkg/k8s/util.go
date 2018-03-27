package k8s

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// PodIsDaemonSet returns if the pod is a daemonset or not
func PodIsDaemonSet(pod *v1.Pod) bool {
	for _, ownerrefence := range pod.ObjectMeta.OwnerReferences {
		if ownerrefence.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}

// CalculatePodsRequestsTotal returns the total capacity of all pods
func CalculatePodsRequestsTotal(pods []*v1.Pod) (resource.Quantity, resource.Quantity, error) {
	var memoryRequest resource.Quantity
	var cpuRequests resource.Quantity

	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			memoryRequest.Add(*container.Resources.Requests.Memory())
			cpuRequests.Add(*container.Resources.Requests.Cpu())
		}
	}

	return memoryRequest, cpuRequests, nil
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
