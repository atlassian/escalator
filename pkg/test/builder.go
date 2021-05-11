package test

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
)

// NodeOpts minimal options for configuring a node object in testing
type NodeOpts struct {
	Name       string
	CPU        int64
	Mem        int64
	LabelKey   string
	LabelValue string
	Creation   time.Time
	Tainted    bool
}

// BuildFakeClient creates a fake client
func BuildFakeClient(nodes []*apiv1.Node, pods []*apiv1.Pod) (*fake.Clientset, <-chan string) {
	fakeClient := &fake.Clientset{}
	updateChan := make(chan string, 2*(len(nodes)+len(pods)))
	// nodes
	fakeClient.Fake.AddReactor("get", "nodes", func(action core.Action) (bool, runtime.Object, error) {
		getAction := action.(core.GetAction)
		for _, node := range nodes {
			if node.Name == getAction.GetName() {
				return true, node, nil
			}
		}
		return true, nil, fmt.Errorf("No node named: %v", getAction.GetName())
	})
	fakeClient.Fake.AddReactor("update", "nodes", func(action core.Action) (bool, runtime.Object, error) {
		updateAction := action.(core.UpdateAction)
		node := updateAction.GetObject().(*apiv1.Node)
		for _, n := range nodes {
			if node.Name == n.Name {
				updateChan <- node.Name
				return true, node, nil
			}
		}
		return false, nil, fmt.Errorf("No node named: %v", node.Name)
	})
	fakeClient.Fake.AddReactor("list", "nodes", func(action core.Action) (bool, runtime.Object, error) {
		nodesCopy := make([]apiv1.Node, 0, len(nodes))
		for _, n := range nodes {
			nodesCopy = append(nodesCopy, *n)
		}
		return true, &apiv1.NodeList{Items: nodesCopy}, nil
	})

	// pods
	fakeClient.Fake.AddReactor("get", "pods", func(action core.Action) (bool, runtime.Object, error) {
		getAction := action.(core.GetAction)
		for _, pod := range pods {
			if pod.Name == getAction.GetName() && pod.Namespace == getAction.GetNamespace() {
				return true, pod, nil
			}
		}
		return true, nil, fmt.Errorf("No pod named: %v", getAction.GetName())
	})
	fakeClient.Fake.AddReactor("update", "pods", func(action core.Action) (bool, runtime.Object, error) {
		updateAction := action.(core.UpdateAction)
		pod := updateAction.GetObject().(*apiv1.Pod)
		for _, p := range pods {
			if pod.Name == p.Name {
				updateChan <- pod.Name
				return true, pod, nil
			}
		}
		return false, nil, fmt.Errorf("No pod named: %v", pod.Name)
	})
	fakeClient.Fake.AddReactor("list", "pods", func(action core.Action) (bool, runtime.Object, error) {
		podsCopy := make([]apiv1.Pod, 0, len(pods))
		for _, p := range pods {
			podsCopy = append(podsCopy, *p)
		}
		return true, &apiv1.PodList{Items: podsCopy}, nil
	})
	return fakeClient, updateChan
}

// NameFromChan returns a name from a channel update
// fails if timeout
func NameFromChan(c <-chan string, timeout time.Duration) string {
	select {
	case val := <-c:
		return val
	case <-time.After(timeout):
		return "Nothing returned"
	}
}

// BuildTestNode creates a node with specified capacity.
func BuildTestNode(opts NodeOpts) *apiv1.Node {

	var taints []apiv1.Taint
	if opts.Tainted {
		taints = append(taints, apiv1.Taint{
			Key:    "atlassian.com/escalator",
			Value:  fmt.Sprint(time.Now().Unix()),
			Effect: apiv1.TaintEffectNoSchedule,
		})
	}

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
			Taints:     taints,
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

// BuildTestNodes creates multiple nodes with the same options
func BuildTestNodes(amount int, opts NodeOpts) []*apiv1.Node {
	var nodes []*apiv1.Node
	for i := 0; i < amount; i++ {
		opts.Name = uuid.New().String()
		nodes = append(nodes, BuildTestNode(opts))
	}
	return nodes
}

// PodOpts are options for a pod
type PodOpts struct {
	Name              string
	Namespace         string
	CPU               []int64
	Mem               []int64
	NodeSelectorKey   string
	NodeSelectorValue string
	Owner             string
	NodeAffinityKey   string
	NodeAffinityValue string
	NodeAffinityOp    v1.NodeSelectorOperator
	NodeName          string
	CPUOverhead       int64
	MemOverhead       int64
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

	var owners []metav1.OwnerReference
	if len(opts.Owner) > 0 {
		owners = append(owners, metav1.OwnerReference{
			Kind: opts.Owner,
		})
	}

	var nodeSelector map[string]string
	if len(opts.NodeSelectorKey) > 0 || len(opts.NodeSelectorValue) > 0 {
		nodeSelector = map[string]string{
			opts.NodeSelectorKey: opts.NodeSelectorValue,
		}
	}

	var affinity *apiv1.Affinity
	if len(opts.NodeAffinityKey) > 0 || len(opts.NodeAffinityValue) > 0 {
		if opts.NodeAffinityOp == "" {
			opts.NodeAffinityOp = v1.NodeSelectorOpIn
		}
		affinity = &apiv1.Affinity{
			NodeAffinity: &apiv1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &apiv1.NodeSelector{
					NodeSelectorTerms: []apiv1.NodeSelectorTerm{
						{
							MatchExpressions: []apiv1.NodeSelectorRequirement{
								{
									Key: opts.NodeAffinityKey,
									Values: []string{
										opts.NodeAffinityValue,
									},
									Operator: opts.NodeAffinityOp,
								},
							},
						},
					},
				},
			},
		}
	}

	overhead := v1.ResourceList{}
	if opts.CPUOverhead > 0 {
		overhead[v1.ResourceCPU] = *resource.NewMilliQuantity(opts.CPUOverhead, resource.DecimalSI)
	}
	if opts.MemOverhead > 0 {
		overhead[v1.ResourceMemory] = *resource.NewQuantity(opts.MemOverhead, resource.DecimalSI)
	}

	pod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       opts.Namespace,
			Name:            opts.Name,
			SelfLink:        fmt.Sprintf("/api/v1/namespaces/%s/pods/%s", opts.Namespace, opts.Name),
			OwnerReferences: owners,
		},
		Spec: apiv1.PodSpec{
			Containers:   containers,
			NodeSelector: nodeSelector,
			Affinity:     affinity,
			Overhead:     overhead,
		},
	}

	if len(opts.NodeName) > 0 {
		pod.Spec.NodeName = opts.NodeName
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

// BuildTestPods creates multiple pods with the same options
func BuildTestPods(amount int, opts PodOpts) []*apiv1.Pod {
	var pods []*apiv1.Pod
	for i := 0; i < amount; i++ {
		opts.Name = fmt.Sprintf("p%d", i)
		pods = append(pods, BuildTestPod(opts))
	}
	return pods
}
