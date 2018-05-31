package k8s

import (
	"k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/autoscaler/cluster-autoscaler/simulator"
	"k8s.io/client-go/kubernetes"
	"time"
)

// DrainPods attempts to delete or evict pods so that the node can be terminated.
// Will prioritise using Evict if the API server supports it.
func DrainPods(pods []*v1.Pod, client kubernetes.Interface, nodeGroupName string) error {
	// Determine whether we are able to delete or evict pods
	apiVersion, err := SupportEviction(client)
	if err != nil {
		return err
	}

	// If we are able to evict
	if len(apiVersion) > 0 {
		return EvictPods(pods, apiVersion, client, nodeGroupName)
	} else {
		// Otherwise delete pods
		return DeletePods(pods, client, nodeGroupName)
	}
}

// CalculateMostDrainableNodes performs a simulation to determine what nodes can be safely removed and the amount of
// pods on each node that will need to be rescheduled as a result of this.
// We then use this to prioritise which nodes should be selected first for termination based on the amount of pods we
// need to reschedule.
func CalculateMostDrainableNodes(candidateNodes []*v1.Node, allNodes []*v1.Node, pods []*v1.Pod, client kubernetes.Interface) (nodesToRemove []simulator.NodeToBeRemoved, err error) {
	predicateChecker, err := simulator.NewPredicateChecker(client, make(<-chan struct{}))
	usageTracker := simulator.NewUsageTracker()
	pdbs, err := getPodDisruptionBudgets(client)
	if err != nil {
		return nodesToRemove, err
	}

	nodesToRemove, _, _, err = simulator.FindNodesToRemove(
		candidateNodes,
		allNodes,
		pods,
		client,
		predicateChecker,
		len(candidateNodes),
		false,
		map[string]string{},
		usageTracker,
		time.Now(),
		pdbs,
	)

	return nodesToRemove, err
}

// getPodDisruptionBudgets gets all pod disruption budgets from all namespaces
func getPodDisruptionBudgets(client kubernetes.Interface) (pdbs []*v1beta1.PodDisruptionBudget, err error) {
	pdbList, err := client.PolicyV1beta1().PodDisruptionBudgets("").List(metaV1.ListOptions{})
	if err != nil {
		return pdbs, err
	}
	for _, item := range pdbList.Items {
		pdbs = append(pdbs, &item)
	}
	return pdbs, nil
}
