package controller

import (
	"fmt"
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
		// TODO(jgonzalez): elaborate
		log.Warningln("Reaping nodes went bad", err)
		// continue instead of exiting, because reaping nodes is separate than tainting
	}
	log.Infoln("There were", removed, "nodes removed this round")

	tainted, err := c.scaleDownTaint(opts)
	return tainted, err
}

// TryRemoveTaintedNodes attempts to remove nodes are tainted and empty or have passed their grace period
func (c *Controller) TryRemoveTaintedNodes(opts scaleOpts) (int, error) {
	return 0, nil
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

	// Lock the taintinf to a maximum on 10 nodes
	if err := k8s.BeginTaintFailSafe(nodesToRemove); err != nil {
		// Don't taint if there was an error on the lock
		log.Errorf("Failed to get safetly lock on tainter: %v", err)
		return 0, err
	}
	// Perform the tainting loop with the fail safe around it
	tainted := c.taintOldestN(opts.untaintedNodes, opts.nodeGroup, nodesToRemove)
	// Validate the Failsafe worked
	if err := k8s.EndTaintFailSafe(len(tainted)); err != nil {
		log.Errorf("Failed to validate safetly lock on tainter: %v", err)
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
			log.WithField("drymode", "off").Infoln("Tainting node", bundle.node.Name)

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
			log.WithField("drymode", "on").Infoln("Tainting node", bundle.node.Name)
		}
	}

	return taintedIndices
}
