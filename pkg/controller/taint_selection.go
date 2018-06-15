package controller

import (
	"github.com/atlassian/escalator/pkg/k8s"
	"k8s.io/api/core/v1"
	"log"
	"sort"
)

func (c *Controller) selectNodesToTaint(untainted []*v1.Node, allNodes []*v1.Node, pods []*v1.Pod, nodeGroup *NodeGroupState) ([]nodeIndexBundle, error) {
	results := make(map[string]map[string]float64)

	// For each taint selection method, run it, apply the weighting to the result and add to the result bundle
	for method, weighting := range nodeGroup.Opts.TaintSelectionMethods {
		results[method] = make(map[string]float64)
		var result []nodeIndexBundle
		var err error

		// Perform the selection method
		switch method {
		case "drainable":
			result, err = c.taintDrainable(untainted, allNodes, pods)
			break
		case "oldest":
			result, err = c.taintOldest(untainted)
			break
		}
		if err != nil {
			return []nodeIndexBundle{}, err
		}

		// Save the results
		for i, n := range result {
			results[method][n.node.Name] = float64(i+1) * weighting
		}
	}

	// Aggregate the scores together
	sorted := make(nodesByLowestScore, 0, len(untainted))
OUTER:
	for i, n := range untainted {
		score := float64(0)
		for _, resultBundles := range results {
			result, ok := resultBundles[n.Name]
			if !ok {
				continue OUTER
			}
			score += result
		}
		sorted = append(sorted, nodeIndexBundle{
			node:  n,
			index: i,
			score: score,
		})
	}
	sort.Sort(sorted)
	return sorted, nil
}

func (c *Controller) taintDrainable(untaintedNodes []*v1.Node, allNodes []*v1.Node, pods []*v1.Pod) ([]nodeIndexBundle, error) {
	// Perform a drain simulation on all the nodes to determine which nodes can be removed
	nodesToBeRemoved, err := k8s.CalculateMostDrainableNodes(untaintedNodes, allNodes, pods, c.Client)
	if err != nil {
		return []nodeIndexBundle{}, err
	}

	// Sort the nodes to be removed by the least amount of pods to be removed
	sorted := make(nodesByPodsToRescheduleLeast, 0, len(nodesToBeRemoved))
	for i, nodeToBeRemoved := range nodesToBeRemoved {
		sorted = append(sorted, nodeIndexBundle{
			node:  nodeToBeRemoved.Node,
			index: i,
			pods:  nodeToBeRemoved.PodsToReschedule,
		})
	}
	sort.Sort(sorted)

	log.Println("taintDrainable")
	log.Println(len(untaintedNodes))
	log.Println(len(sorted))

	return sorted, nil
}

// taintOldest sorts nodes by creation time and then taints the oldest nodes.
func (c *Controller) taintOldest(nodes []*v1.Node) ([]nodeIndexBundle, error) {
	// Add each node to the sort struct
	sorted := make(nodesByOldestCreationTime, 0, len(nodes))
	for i, node := range nodes {
		sorted = append(sorted, nodeIndexBundle{node: node, index: i})
	}
	sort.Sort(sorted)
	return sorted, nil
}
