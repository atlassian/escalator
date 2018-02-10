package test

import (
	"fmt"
	"time"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeOpts minimal options for configuring a node object in testing
type NodeOpts struct {
	Name       string
	CPU        int64
	Mem        int64
	LabelKey   string
	LabelValue string
	Creation   time.Time
}

// BuildTestNode creates a node with specified capacity.
func BuildTestNode(opts NodeOpts) *apiv1.Node {
	node := &apiv1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:     opts.Name,
			SelfLink: fmt.Sprintf("/api/v1/nodes/%s", opts.Name),
			Labels: map[string]string{
				opts.LabelKey: opts.LabelValue,
			},
			CreationTimestamp: metav1.NewTime(opts.Creation),
		},
		Spec: apiv1.NodeSpec{
			ProviderID: opts.Name,
		},
		Status: apiv1.NodeStatus{
			Capacity: apiv1.ResourceList{
				apiv1.ResourcePods: *resource.NewQuantity(100, resource.DecimalSI),
			},
		},
	}

	if opts.CPU >= 0 {
		node.Status.Capacity[apiv1.ResourceCPU] = *resource.NewMilliQuantity(opts.CPU, resource.DecimalSI)
	}
	if opts.Mem >= 0 {
		node.Status.Capacity[apiv1.ResourceMemory] = *resource.NewQuantity(opts.Mem, resource.DecimalSI)
	}

	node.Status.Allocatable = apiv1.ResourceList{}
	for k, v := range node.Status.Capacity {
		node.Status.Allocatable[k] = v
	}

	return node
}

// PodOpts are options for a pod
type PodOpts struct {
	Name              string
	Namespace         string
	CPU               []int64
	Mem               []int64
	NodeSelectorKey   string
	NodeSelectorValue string
}

// BuildTestPod builds a pod for testing
func BuildTestPod(opts PodOpts) *apiv1.Pod {
	containers := make([]apiv1.Container, 0, len(opts.CPU))
	for range opts.CPU {
		containers = append(containers, apiv1.Container{
			Resources: apiv1.ResourceRequirements{
				Requests: apiv1.ResourceList{},
			},
		})
	}

	pod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: opts.Namespace,
			Name:      opts.Name,
			SelfLink:  fmt.Sprintf("/api/v1/namespaces/%s/pods/%s", opts.Namespace, opts.Name),
		},
		Spec: apiv1.PodSpec{
			Containers: containers,
			NodeSelector: map[string]string{
				opts.NodeSelectorKey: opts.NodeSelectorValue,
			},
		},
	}

	for i := range containers {
		if opts.CPU[i] >= 0 {
			pod.Spec.Containers[i].Resources.Requests[apiv1.ResourceCPU] = *resource.NewMilliQuantity(opts.CPU[i], resource.DecimalSI)
		}
		if opts.Mem[i] >= 0 {
			pod.Spec.Containers[i].Resources.Requests[apiv1.ResourceMemory] = *resource.NewQuantity(opts.Mem[i], resource.DecimalSI)
		}
	}

	return pod
}
