package controller

import (
	"fmt"
	"sort"

	"github.com/atlassian/escalator/pkg/k8s"
	"github.com/atlassian/escalator/pkg/metrics"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

// ScaleUp performs the untaint and increase cloud provider node group logic
func (c *Controller) ScaleUp(opts scaleOpts) (int, error) {

	untainted, err := c.scaleUpUntaint(opts)
	// No nodes were untainted, so we need to scale up cloud provider node group
	if err != nil {
		log.Errorf("Failed to untaint nodes because of an error. Skipping cloud provider node group scaleup: %v", err)
		return untainted, err
	}

	// remove the number of nodes that were just untainted and the remaining is how much to increase the cloud provider node group by
	opts.nodesDelta -= untainted

	if opts.nodesDelta > 0 {
		// check that untainting the nodes doesn't do bring us over max nodes
		if opts.nodesDelta <= 0 {
			log.Warnf("Scale up delta is less than or equal to 0 after clamping: %v. Will not scale up cloud provider.", opts.nodesDelta)
			return 0, nil
		}

		if opts.nodesDelta > 0 {
			added, err := c.scaleUpCloudProviderNodeGroup(opts)
			if err != nil {
				log.Errorf("Failed to add nodes because of an error. Skipping cloud provider node group scaleup: %v", err)
				return 0, err
			}
			opts.nodeGroup.scaleUpLock.lock(added)
			return untainted + added, nil
		}
	}

	return untainted, nil
}

// Calulates how many new nodes need to be created
func (c *Controller) calculateNodesToAdd(nodesToAdd int64, TargetSize int64, MaxNodes int64) int64 {
	// Clamp it to the max if exceeding max target size
	if TargetSize+nodesToAdd > MaxNodes {
		nodesToAdd = MaxNodes - TargetSize
		log.Infof("increasing nodes exceeds maximum (%v). Clamping add amount to (%v)", MaxNodes, nodesToAdd)
	}
	return nodesToAdd
}

// scaleUpCloudProviderNodeGroup increases the size of the cloud provider node group by opts.nodesDelta
func (c *Controller) scaleUpCloudProviderNodeGroup(opts scaleOpts) (int, error) {

	cloudProviderNodeGroup, ok := c.cloudProvider.GetNodeGroup(opts.nodeGroup.Opts.CloudProviderGroupName)
	if !ok {
		return 0, fmt.Errorf("cloud provider node group does not exist: %s", opts.nodeGroup.Opts.CloudProviderGroupName)
	}

	nodegroupName := opts.nodeGroup.Opts.Name
	nodesToAdd := c.calculateNodesToAdd(int64(opts.nodesDelta), cloudProviderNodeGroup.TargetSize(), cloudProviderNodeGroup.MaxSize())
	if nodesToAdd <= 0 {
		err := fmt.Errorf(
			"refusing to scaleup up beyond the maximum size of the autoscaling group (TargetSize: %v; MaxNodes: %v). Taking no action",
			cloudProviderNodeGroup.TargetSize(),
			opts.nodeGroup.Opts.MaxNodes,
		)
		log.WithError(err).Error("Cancelling scaleup")
		return 0, err
	}

	if nodesToAdd > 0 {
		drymode := c.dryMode(opts.nodeGroup)
		log.WithField("drymode", drymode).
			WithField("nodegroup", nodegroupName).
			Infof("increasing cloud provider node group by %v", nodesToAdd)

		if !drymode {
			err := cloudProviderNodeGroup.IncreaseSize(nodesToAdd)
			if err != nil {
				log.Errorf("failed to set cloud provider node group size: %v", err)
				return 0, err
			}
		}
	} else {
		return 0, fmt.Errorf("adding %v nodes would breach max cloud provider node group size (%v)", nodesToAdd, cloudProviderNodeGroup.MaxSize())
	}

	return int(nodesToAdd), nil
}

// scaleUpUntaint tries to untaint opts.nodesDelta nodes
func (c *Controller) scaleUpUntaint(opts scaleOpts) (int, error) {
	nodegroupName := opts.nodeGroup.Opts.Name
	nodesToAdd := opts.nodesDelta

	if len(opts.taintedNodes) == 0 {
		log.WithField("nodegroup", nodegroupName).Warning("There are no tainted nodes to untaint")
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
				log.WithField("drymode", "off").Infof("Untainting node %v", bundle.node.Name)

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
				log.WithField("drymode", "on").Infof("Untainting node %v", bundle.node.Name)
			}
		}
	}

	return untaintedIndices
}
