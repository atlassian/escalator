package controller

import (
	"fmt"
	"sort"

	"github.com/atlassian/escalator/pkg/cloudprovider/aws"
	"github.com/atlassian/escalator/pkg/k8s"
	"github.com/atlassian/escalator/pkg/metrics"
	log "github.com/sirupsen/logrus"
	time "github.com/stephanos/clock"
	"k8s.io/api/core/v1"
)

// ScaleDown performs the taint and remove node logic
func (c *Controller) ScaleDown(opts scaleOpts) (int, error) {
	removed, err := c.TryRemoveTaintedNodes(opts)
	if err != nil {
		switch err.(type) {
		// early return when node not in expected autoscaling group is found
		case *aws.NodeNotInAutoScalingGroup:
			return 0, err
		default:
			// continue instead of exiting, because reaping nodes is separate than tainting
			log.WithError(err).Warning("Reaping nodes failed")
		}
	}
	log.Infof("Reaper: There were %v empty nodes deleted this round", removed)
	return c.scaleDownTaint(opts)
}

// TryRemoveTaintedNodes attempts to remove nodes are tainted and empty or have passed their grace period
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
			if k8s.NodeEmpty(candidate, opts.nodeGroup.NodeInfoMap) || now.Sub(*taintedTime) > opts.nodeGroup.Opts.HardDeleteGracePeriodDuration() {
				drymode := c.dryMode(opts.nodeGroup)
				log.WithField("drymode", drymode).Infof("Node %v, %v ready to be deleted", candidate.Name, candidate.Spec.ProviderID)
				if !drymode {
					toBeDeleted = append(toBeDeleted, candidate)
				}
			} else {
				nodePodsRemaining, ok := k8s.NodePodsRemaining(candidate, opts.nodeGroup.NodeInfoMap)
				podsRemainingMessage := ""
				if ok {
					podsRemainingMessage = fmt.Sprintf("%d pods remaining", nodePodsRemaining)
				} else {
					podsRemainingMessage = "unknown number of pods remaining"
				}
				log.Debugf("node %v not ready for deletion (%s). Hard delete time remaining %v",
					candidate.Name,
					podsRemainingMessage,
					opts.nodeGroup.Opts.HardDeleteGracePeriodDuration()-now.Sub(*taintedTime),
				)
			}
		} else {
			log.Debugf("node %v not ready for deletion yet. Time remaining %v",
				candidate.Name,
				opts.nodeGroup.Opts.SoftDeleteGracePeriodDuration()-now.Sub(*taintedTime),
			)
		}
	}

	if len(toBeDeleted) > 0 {
		podsRemaining := 0
		for _, nodeToBeDeleted := range toBeDeleted {
			nodePodsRemaining, ok := k8s.NodePodsRemaining(nodeToBeDeleted, opts.nodeGroup.NodeInfoMap)
			if !ok {
				continue
			}

			podsRemaining += nodePodsRemaining
		}

		cloudProviderNodeGroup, ok := c.cloudProvider.GetNodeGroup(opts.nodeGroup.Opts.CloudProviderGroupName)
		if !ok {
			return 0, fmt.Errorf("cloud provider node group does not exist: %s", opts.nodeGroup.Opts.CloudProviderGroupName)
		}

		// Terminate the nodes in the cloud provider
		err := cloudProviderNodeGroup.DeleteNodes(toBeDeleted...)
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
		metrics.NodeGroupPodsEvicted.WithLabelValues(opts.nodeGroup.Opts.Name).Add(float64(podsRemaining))
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

	log.WithField("nodegroup", nodegroupName).Infof("Scaling Down: tainting %v nodes", nodesToRemove)
	metrics.NodeGroupTaintEvent.WithLabelValues(nodegroupName).Add(float64(nodesToRemove))

	// Lock the tainting to a maximum on 10 nodes
	if err := k8s.BeginTaintFailSafe(nodesToRemove); err != nil {
		// Don't taint if there was an error on the lock
		log.Errorf("Failed to get safety lock on tainter: %v", err)
		return 0, err
	}
	// Perform the tainting loop with the fail safe around it
	tainted := c.taintOldestN(opts.untaintedNodes, opts.nodeGroup, nodesToRemove)
	// Validate the fail-safe worked
	if err := k8s.EndTaintFailSafe(len(tainted)); err != nil {
		log.Errorf("Failed to validate safety lock on tainter: %v", err)
		return len(tainted), err
	}

	log.Infof("Tainted a total of %v nodes", len(tainted))
	return len(tainted), nil
}

// taintOldestN sorts nodes by creation time and taints the oldest N. It will return an array of indices of the nodes it tainted
// indices are from the parameter nodes indexes, not the sorted index
func (c *Controller) taintOldestN(nodes []*v1.Node, nodeGroup *NodeGroupState, n int) []int {
	sorted := make(nodesByOldestCreationTime, 0, len(nodes))
	for i, node := range nodes {
		sorted = append(sorted, nodeIndexBundle{node, i})
	}
	sort.Sort(sorted)

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
