package controller

import (
	"fmt"
	time "github.com/stephanos/clock"
	"sort"

	"github.com/atlassian/escalator/pkg/k8s"
	"github.com/atlassian/escalator/pkg/metrics"
	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
)

// ScaleDown performs the taint and remove node logic
func (c *Controller) ScaleDown(opts scaleOpts) (int, error) {
	removed, err := c.TryRemoveTaintedNodes(opts)
	if err != nil {
		// continue instead of exiting, because reaping nodes is separate than tainting
		log.WithError(err).Warning("Reaping nodes failed")
	}
	log.Infof("Reaper: There were %v empty nodes deleted this round", removed)
	return c.scaleDownTaint(opts)
}

// TryRemoveTaintedNodes attempts to remove nodes are tainted and empty or have passed their grace period
// TODO(aprice): clean up this method (lots of IF statements)
func (c *Controller) TryRemoveTaintedNodes(opts scaleOpts) (int, error) {
	var toBeDeleted []*v1.Node
	for _, candidate := range opts.taintedNodes {
		// if the time the node was tainted is larger than the hard period then it is deleted no matter what
		// if the soft time is passed and the node is empty (excluding daemonsets) then it can be deleted
		taintedTime, err := k8s.GetToBeRemovedTime(candidate)
		if err != nil || taintedTime == nil {
			log.WithError(err).Errorf("unable to get tainted time from node %v. Ignore if running in drymode", candidate.Name)
			continue
		}

		now := time.Now()
		if now.Sub(*taintedTime) > opts.nodeGroup.Opts.SoftDeleteGracePeriodDuration() {
			// If draining is enabled, attempt to drain the node
			if opts.nodeGroup.Opts.DrainBeforeTermination && !k8s.NodeEmpty(candidate, opts.nodeGroup.NodeInfoMap) {
				podsForDeletion, err := k8s.NodeGetPodsForDeletion(candidate, opts.nodeGroup.NodeInfoMap)
				if err != nil {
					return 0, err
				}

				err = k8s.DrainPods(podsForDeletion, c.Client, opts.nodeGroup.Opts.Name)
				if err != nil {
					return 0, err
				}
			}

			shouldHardDelete := opts.nodeGroup.Opts.ShouldHardDelete()
			if k8s.NodeEmpty(candidate, opts.nodeGroup.NodeInfoMap) || (shouldHardDelete && now.Sub(*taintedTime) > opts.nodeGroup.Opts.HardDeleteGracePeriodDuration()) {
				drymode := c.dryMode(opts.nodeGroup)
				log.WithField("drymode", drymode).Infof("Node %v, %v ready to be deleted", candidate.Name, candidate.Spec.ProviderID)
				if !drymode {
					toBeDeleted = append(toBeDeleted, candidate)
				}
			} else {
				if shouldHardDelete {
					log.Debugf("node %v not ready for deletion. Hard delete time remaining %v",
						candidate.Name,
						opts.nodeGroup.Opts.HardDeleteGracePeriodDuration()-now.Sub(*taintedTime),
					)
				} else {
					log.Debugf("node %v not ready for deletion. Won't hard delete.", candidate.Name)
				}
			}
		} else {
			log.Debugf("node %v not ready for deletion yet. Time remaining %v",
				candidate.Name,
				opts.nodeGroup.Opts.SoftDeleteGracePeriodDuration()-now.Sub(*taintedTime),
			)
		}
	}

	if len(toBeDeleted) > 0 {
		// Terminate the nodes in the cloud provider
		err := opts.nodeGroup.CloudProviderNodeGroup.DeleteNodes(toBeDeleted...)
		if err != nil {
			for _, nodeToDelete := range toBeDeleted {
				log.WithError(err).Errorf("failed to terminate node in cloud provider %v, %v", nodeToDelete.Name, nodeToDelete.Spec.ProviderID)
			}
			return 0, err
		}

		// Delete the nodes from kubernetes
		err = k8s.DeleteNodes(toBeDeleted, c.Client)
		if err != nil {
			log.WithError(err).Errorf("failed to delete nodes from kubernetes")
			return 0, err
		}
		log.Infof("Sent delete request to %v nodes", len(toBeDeleted))
	}

	return -len(toBeDeleted), nil
}

func (c *Controller) scaleDownTaint(opts scaleOpts) (int, error) {
	nodegroupName := opts.nodeGroup.Opts.Name
	nodesToRemove := opts.nodesDelta

	// Clamp the scale down so it doesn't drop under the min nodes
	if len(opts.untaintedNodes)-nodesToRemove < opts.nodeGroup.Opts.MinNodes {
		// Set the delta to maximum amount we can remove without going over
		nodesToRemove = len(opts.untaintedNodes) - opts.nodeGroup.Opts.MinNodes
		log.Infof("untainted nodes close to minimum (%v). Adjusting taint amount to (%v)", opts.nodeGroup.Opts.MinNodes, nodesToRemove)
		// If have less node than the minimum, abort!
		if nodesToRemove < 0 {
			err := fmt.Errorf(
				"the number of nodes(%v) is less than specified minimum of %v. Taking no action",
				len(opts.untaintedNodes),
				opts.nodeGroup.Opts.MinNodes,
			)
			log.WithError(err).Error("Cancelling scaledown")
			return 0, err
		}
	}

	log.WithFields(log.Fields{
		"nodegroup":               nodegroupName,
		"taint_selection_methods": fmt.Sprintf("%v", opts.nodeGroup.Opts.TaintSelectionMethods),
	}).Infof("Scaling Down: tainting %v nodes", nodesToRemove)
	metrics.NodeGroupTaintEvent.WithLabelValues(nodegroupName).Add(float64(nodesToRemove))

	// Don't bother selecting nodes to taint if we aren't removing any
	if nodesToRemove == 0 {
		return 0, nil
	}

	// Lock the tainting to a maximum on 10 nodes
	if err := k8s.BeginTaintFailSafe(nodesToRemove); err != nil {
		// Don't taint if there was an error on the lock
		log.Errorf("Failed to get safety lock on tainter: %v", err)
		return 0, err
	}

	// Perform the tainting loop with the fail safe around it
	var tainted []int
	var err error
	taintSelectionMethods := opts.nodeGroup.Opts.TaintSelectionMethods
	if taintSelectionMethods[0] == "oldest" {
		tainted, err = c.taintOldest(opts.untaintedNodes, opts.nodeGroup, nodesToRemove)
	} else if taintSelectionMethods[0] == "drainable" {
		tainted, err = c.taintDrainable(opts.untaintedNodes, opts.nodes, opts.pods, opts.nodeGroup, nodesToRemove)
	}

	if err != nil {
		log.Errorf("Failed to select and taint nodes: %v", err)
	}

	// Validate the fail-safe worked
	if err := k8s.EndTaintFailSafe(len(tainted)); err != nil {
		log.Errorf("Failed to validate safety lock on tainter: %v", err)
		return len(tainted), err
	}

	log.Infof("Tainted a total of %v nodes", len(tainted))
	return len(tainted), nil
}

func (c *Controller) taintDrainable(untaintedNodes []*v1.Node, allNodes []*v1.Node, pods []*v1.Pod, nodeGroup *NodeGroupState, n int) ([]int, error) {
	// Perform a drain simulation on all the nodes to determine which nodes can be removed
	nodesToBeRemoved, err := k8s.CalculateMostDrainableNodes(untaintedNodes, allNodes, pods, c.Client)
	if err != nil {
		return []int{}, err
	}

	// Sort the nodes to be removed by the least amount of pods to be removed
	sorted := make(nodesByPodsToRescheduleLeast, 0, len(nodesToBeRemoved))
	for i, nodeToBeRemoved := range nodesToBeRemoved {
		sorted = append(sorted, nodeIndexBundle{
			nodeToBeRemoved.Node,
			i,
			nodeToBeRemoved.PodsToReschedule,
		})
	}
	sort.Sort(sorted)

	return c.taintNodes(sorted, nodeGroup, n), nil
}

// taintOldest sorts nodes by creation time and then taints the oldest nodes.
func (c *Controller) taintOldest(nodes []*v1.Node, nodeGroup *NodeGroupState, n int) ([]int, error) {
	// Add each node to the sort struct
	sorted := make(nodesByOldestCreationTime, 0, len(nodes))
	for i, node := range nodes {
		sorted = append(sorted, nodeIndexBundle{node, i, []*v1.Pod{}})
	}
	sort.Sort(sorted)
	return c.taintNodes(sorted, nodeGroup, n), nil
}

// taintNodes taints nodes in the order of the nodes parameter.
// It will return an array of indices of the nodes it tainted.
// Indices are from the parameter nodes indexes, not the sorted index.
func (c *Controller) taintNodes(sorted []nodeIndexBundle, nodeGroup *NodeGroupState, n int) []int {
	taintedIndices := make([]int, 0, n)
	for i, bundle := range sorted {
		// stop at N (or when array is fully iterated)
		if len(taintedIndices) >= n || i >= k8s.MaximumTaints {
			break
		}

		// only actually taint in dry mode
		if !c.dryMode(nodeGroup) {
			log.WithField("drymode", "off").Infof("Tainting node %v", bundle.node.Name)

			// Taint the node
			updatedNode, err := k8s.AddToBeRemovedTaint(bundle.node, c.Client)
			if err != nil {
				log.Errorf("While tainting %v: %v", bundle.node.Name, err)
			} else {
				bundle.node = updatedNode
				taintedIndices = append(taintedIndices, bundle.index)
			}
		} else {
			nodeGroup.taintTracker = append(nodeGroup.taintTracker, bundle.node.Name)
			k8s.IncrementTaintCount()
			taintedIndices = append(taintedIndices, bundle.index)
			log.WithField("drymode", "on").Infof("Tainting node %v", bundle.node.Name)
		}
	}

	return taintedIndices
}
