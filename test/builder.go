package test

import (
	"fmt"

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
