package k8s

import (
	k8s_resource "github.com/atlassian/escalator/pkg/k8s/resource"
	"github.com/atlassian/escalator/pkg/k8s/scheduler"
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
	memoryRequests := *k8s_resource.NewMemoryQuantity(0)
	cpuRequests := *k8s_resource.NewCPUQuantity(0)

	for _, pod := range pods {
		podResources := scheduler.ComputePodResourceRequest(pod)
		memoryRequests.Add(*k8s_resource.NewMemoryQuantity(podResources.Memory))
		cpuRequests.Add(*k8s_resource.NewCPUQuantity(podResources.MilliCPU))
	}

	return memoryRequests, cpuRequests, nil
}

// CalculateNodesCapacityTotal calculates the total Allocatable node capacity for all nodes
func CalculateNodesCapacityTotal(nodes []*v1.Node) (resource.Quantity, resource.Quantity, error) {
	memoryCapacity := *k8s_resource.NewMemoryQuantity(0)
	cpuCapacity := *k8s_resource.NewCPUQuantity(0)

	for _, node := range nodes {
		memoryCapacity.Add(*node.Status.Allocatable.Memory())
		cpuCapacity.Add(*node.Status.Allocatable.Cpu())
	}

	return memoryCapacity, cpuCapacity, nil
}
