package controller

import (
	"fmt"
	"sort"

	"github.com/atlassian/escalator/pkg/k8s"
	"github.com/atlassian/escalator/pkg/metrics"
	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
)

// ScaleUp performs the untaint and incrase asg logic
func (c *Controller) ScaleUp(opts scaleOpts) (int, error) {
	untainted, err := c.scaleUpUntaint(opts)
	// No nodes were untainted, so we need to scale up asg
	if err != nil {
		log.Error("Failed to untaint nodes because of an error. Skipping ASG scaleup")
		return untainted, err
	}

	// remove the number of nodes that were just untainted and the remaining is how much to increase the asg by
	opts.nodesDelta -= untainted
	if opts.nodesDelta > 0 {
		// TODO(jgonzalez): finish this we return values and such
		c.scaleUpASG(opts)
	}

	return untainted, err
}

func (c *Controller) scaleUpASG(opts scaleOpts) (int, error) {
	nodegroupName := opts.nodeGroup.Opts.Name
	nodesToAdd := opts.nodesDelta
	log.WithField("nodegroup", nodegroupName).Infoln("Increasing ASG by", nodesToAdd)

	//c.cloudProvider.GetNodeGroup(nodegroupName).IncreaseSize(nodesToAdd)

	return 0, nil
}

func (c *Controller) scaleUpUntaint(opts scaleOpts) (int, error) {
	nodegroupName := opts.nodeGroup.Opts.Name
	nodesToAdd := opts.nodesDelta

	// check that untainting the nodes doesn't do bring us over max nodes
	if len(opts.untaintedNodes)+nodesToAdd > opts.nodeGroup.Opts.MaxNodes {
		// Clamp it to the max we can untaint
		nodesToAdd = opts.nodeGroup.Opts.MaxNodes - len(opts.untaintedNodes)
		log.Infof("untainted nodes close to maximum (%v). Adjusting untaint amount to (%v)", opts.nodeGroup.Opts.MaxNodes, nodesToAdd)
		if nodesToAdd < 0 {
			err := fmt.Errorf(
				"the number of nodes(%v) is more than specified maximum of %v. Taking no action",
				len(opts.untaintedNodes),
				opts.nodeGroup.Opts.MaxNodes,
			)
			log.WithError(err).Error("Cancelling scaleup")
			return 0, err
		}
	}

	if len(opts.taintedNodes) == 0 {
		log.WithField("nodegroup", nodegroupName).Warningln("There are no tainted nodes to untaint")
		return 0, nil
	}

	// Metrics & Logs
	log.WithField("nodegroup", nodegroupName).Infof("Scaling Up: Trying to untaint %v tainted nodes", nodesToAdd)
	metrics.NodeGroupUntaintEvent.WithLabelValues(nodegroupName).Add(float64(nodesToAdd))

	untainted := c.untaintNewestN(opts.taintedNodes, opts.nodeGroup, nodesToAdd)
	log.Infof("Untainted a total of %v nodes", len(untainted))
	return len(untainted), nil
}

// untaintNewestN sorts nodes by creation time and untaints the newest N. It will return an array of indices of the nodes it untainted
// indices are from the parameter nodes indexes, not the sorted index
func (c *Controller) untaintNewestN(nodes []*v1.Node, nodeGroup *NodeGroupState, n int) []int {
	sorted := make(nodesByNewestCreationTime, 0, len(nodes))
	for i, node := range nodes {
		sorted = append(sorted, nodeIndexBundle{node, i})
	}
	sort.Sort(sorted)

	untaintedIndices := make([]int, 0, n)
	for _, bundle := range sorted {
		// stop at N (or when array is fully iterated)
		if len(untaintedIndices) >= n {
			break
		}
		// only actually taint in dry mode
		if !c.dryMode(nodeGroup) {
			if _, tainted := k8s.GetToBeRemovedTaint(bundle.node); tainted {
				log.WithField("drymode", "off").Infoln("Untainting node", bundle.node.Name)

				// Remove the taint from the node
				updatedNode, err := k8s.DeleteToBeRemovedTaint(bundle.node, c.Client)
				if err != nil {
					log.Errorf("Failed to untaint node %v: %v", bundle.node.Name, err)
				} else {
					bundle.node = updatedNode
					untaintedIndices = append(untaintedIndices, bundle.index)
				}
			}
		} else {
			deleteIndex := -1
			for i, name := range nodeGroup.taintTracker {
				if bundle.node.Name == name {
					deleteIndex = i
					break
				}
			}
			if deleteIndex != -1 {
				// Delete from tracker
				nodeGroup.taintTracker = append(nodeGroup.taintTracker[:deleteIndex], nodeGroup.taintTracker[deleteIndex+1:]...)
				untaintedIndices = append(untaintedIndices, bundle.index)
				log.WithField("drymode", "on").Infoln("Untainting node", bundle.node.Name)
			}
		}
	}

	return untaintedIndices
}
