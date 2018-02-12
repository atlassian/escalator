package controller

import (
	"errors"
	"math"
	"sort"
	"time"

	"github.com/atlassian/escalator/pkg/k8s"
	"github.com/atlassian/escalator/pkg/metrics"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"

	log "github.com/sirupsen/logrus"
)

// Controller contains the core logic of the Autoscaler
type Controller struct {
	Client   *Client
	Opts     *Opts
	stopChan <-chan struct{}

	nodeGroups map[string]*NodeGroupState
}

// NodeGroupState contains everything about a node group in the current state of the application
type NodeGroupState struct {
	Opts *NodeGroupOptions
	*NodeGroupLister

	// used for tracking which nodes are tainted. testing when in dry mode
	taintTracker []string
}

// Opts provide the Controller with config for runtime
type Opts struct {
	K8SClient  kubernetes.Interface
	NodeGroups []*NodeGroupOptions

	ScanInterval time.Duration
	DryMode      bool
}

// NewController creates a new controller with the specified options
func NewController(opts *Opts, stopChan <-chan struct{}) *Controller {
	client := NewClient(opts.K8SClient, opts.NodeGroups, stopChan)
	if client == nil {
		log.Fatalln("Failed to create controller client")
		return nil
	}

	// turn it into a map of name and nodegroupstate for O(1) lookup and data bundling
	nodegroupMap := make(map[string]*NodeGroupState)
	for _, nodeGroupOpts := range opts.NodeGroups {
		nodegroupMap[nodeGroupOpts.Name] = &NodeGroupState{
			Opts:            nodeGroupOpts,
			NodeGroupLister: client.Listers[nodeGroupOpts.Name],
		}
	}

	return &Controller{
		Client:     client,
		Opts:       opts,
		stopChan:   stopChan,
		nodeGroups: nodegroupMap,
	}
}

// dryMode is a helper that returns the overall drymode result of the controller and nodegroup
func (c Controller) dryMode(nodeGroup *NodeGroupState) bool {
	return c.Opts.DryMode || nodeGroup.Opts.DryMode
}

// calcPercentUsage helper works out the percentage of cpu and mem for request/capacity
func calcPercentUsage(cpuR, memR, cpuA, memA resource.Quantity) (float64, float64, error) {
	if cpuA.MilliValue() == 0 || memA.MilliValue() == 0 {
		return 0, 0, errors.New("Cannot divide by zero in percent calculation")
	}
	cpuPercent := float64(cpuR.MilliValue()) / float64(cpuA.MilliValue()) * 100
	memPercent := float64(memR.MilliValue()) / float64(memA.MilliValue()) * 100
	return cpuPercent, memPercent, nil
}

// taintOldestN sorts nodes by creation time and taints the oldest N. It will return an array of indices of the nodes it tainted
// indices are from the parameter nodes indexes, not the sorted index
func (c Controller) taintOldestN(nodes []*v1.Node, nodeGroup *NodeGroupState, n int) []int {
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

// untaintNewestN sorts nodes by creation time and untaints the newest N. It will return an array of indices of the nodes it untainted
// indices are from the parameter nodes indexes, not the sorted index
func (c Controller) untaintNewestN(nodes []*v1.Node, nodeGroup *NodeGroupState, n int) []int {
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

// scaleNodeGroup performs the core logic of calculating util and choosig a scaling action for a node group
func (c Controller) scaleNodeGroup(nodegroup string, nodeGroup *NodeGroupState) {
	// list all pods
	pods, err := nodeGroup.Pods.List()
	if err != nil {
		log.Errorf("Failed to list pods: %v", err)
		return
	}

	// List all nodes
	allNodes, err := nodeGroup.Nodes.List()
	if err != nil {
		log.Errorf("Failed to list nodes: %v", err)
		return
	}

	// Filter into untainted and tainted nodes
	untaintedNodes := make([]*v1.Node, 0, len(allNodes))
	taintedNodes := make([]*v1.Node, 0, len(allNodes))
	for _, node := range allNodes {
		if c.dryMode(nodeGroup) {
			var contains bool
			for _, name := range nodeGroup.taintTracker {
				if node.Name == name {
					contains = true
					break
				}
			}
			if !contains {
				untaintedNodes = append(untaintedNodes, node)
			} else {
				taintedNodes = append(taintedNodes, node)
			}
		} else {
			if _, tainted := k8s.GetToBeRemovedTaint(node); !tainted {
				untaintedNodes = append(untaintedNodes, node)
			} else {
				taintedNodes = append(taintedNodes, node)
			}
		}
	}

	// Metrics and Logs
	log.WithField("nodegroup", nodegroup).Infoln("nodes remaining total:", len(allNodes))
	log.WithField("nodegroup", nodegroup).Infoln("nodes remaining untainted:", len(untaintedNodes))
	log.WithField("nodegroup", nodegroup).Infoln("nodes remaining tainted:", len(taintedNodes))
	metrics.NodeGroupNodes.WithLabelValues(nodegroup).Set(float64(len(allNodes)))
	metrics.NodeGroupNodesUntainted.WithLabelValues(nodegroup).Set(float64(len(untaintedNodes)))
	metrics.NodeGroupNodesTainted.WithLabelValues(nodegroup).Set(float64(len(taintedNodes)))
	metrics.NodeGroupPods.WithLabelValues(nodegroup).Set(float64(len(pods)))

	if len(allNodes) == 0 {
		log.WithField("nodegroup", nodegroup).Infoln("no nodes remaining")
		return
	}

	// Calc capacity for untainted nodes
	memRequest, cpuRequest, err := k8s.CalculatePodsRequestsTotal(pods)
	if err != nil {
		log.Errorf("Failed to calculate requests: %v", err)
		return
	}
	memCapacity, cpuCapacity, err := k8s.CalculateNodesCapacityTotal(untaintedNodes)
	if err != nil {
		log.Errorf("Failed to calculate capacity: %v", err)
		return
	}

	// Metrics
	metrics.NodeGroupCPURequest.WithLabelValues(nodegroup).Set(float64(cpuRequest.MilliValue()))
	bytesMemReq, _ := memRequest.AsInt64()
	metrics.NodeGroupMemRequest.WithLabelValues(nodegroup).Set(float64(bytesMemReq))
	metrics.NodeGroupCPUCapacity.WithLabelValues(nodegroup).Set(float64(cpuCapacity.MilliValue()))
	bytesMemCap, _ := memCapacity.AsInt64()
	metrics.NodeGroupMemCapacity.WithLabelValues(nodegroup).Set(float64(bytesMemCap))

	// Calc %
	cpuPercent, memPercent, err := calcPercentUsage(cpuRequest, memRequest, cpuCapacity, memCapacity)
	if err != nil {
		log.Errorf("Failed to calculate percentages: %v", err)
		return
	}

	// Metrics
	log.WithField("nodegroup", nodegroup).Infof("cpu: %v, memory: %v", cpuPercent, memPercent)
	metrics.NodeGroupsCPUPercent.WithLabelValues(nodegroup).Set(cpuPercent)
	metrics.NodeGroupsMemPercent.WithLabelValues(nodegroup).Set(memPercent)

	// Perform the scaling decision
	maxPercent := int(math.Max(cpuPercent, memPercent))
	nodesDelta := 0

	// Determine if we want to scale up for down. Selects the first condition that is true
	switch {
	// --- Scale Down conditions ---
	// reached very low %. aggressively remove nodes
	case maxPercent < nodeGroup.Opts.TaintLowerCapacityThreshholdPercent:
		nodesDelta = -nodeGroup.Opts.FastNodeRemovalRate
	// reached medium low %. slowly remove nodes
	case maxPercent < nodeGroup.Opts.TaintUpperCapacityThreshholdPercent:
		nodesDelta = -nodeGroup.Opts.SlowNodeRemovalRate
	// --- Scale Up conditions ---
	// reached very high %. aggressively add nodes back
	case maxPercent > nodeGroup.Opts.UntaintUpperCapacityThreshholdPercent:
		nodesDelta = nodeGroup.Opts.FastNodeRevivalRate
	// reached medium high %. slowy add nodes back
	case maxPercent > nodeGroup.Opts.UntaintLowerCapacityThreshholdPercent:
		nodesDelta = nodeGroup.Opts.SlowNodeRevivalRate
	}

	log.WithField("nodegroup", nodegroup).Debugln("Delta=", nodesDelta)

	// Clamp the nodes inside the min and max node count
	switch {
	case nodesDelta < 0:
		if len(untaintedNodes)+nodesDelta < nodeGroup.Opts.MinNodes {
			nodesDelta = nodeGroup.Opts.MinNodes - len(untaintedNodes)
			if nodesDelta > 0 {
				log.Warningf(
					"The cluster is under utilised, but the number of nodes(%v) is less than specified minimum of %v. Switching to a scale up.",
					len(untaintedNodes),
					nodeGroup.Opts.MinNodes,
				)
			}
		}
	case nodesDelta > 0:
		if len(untaintedNodes)+nodesDelta > nodeGroup.Opts.MaxNodes {
			nodesDelta = nodeGroup.Opts.MaxNodes - len(untaintedNodes)
			if nodesDelta < 0 {
				log.Warningf(
					"The cluster is over utilised, but the number of nodes(%v) is more than specified maximum of %v. Switching to a scale down.",
					len(untaintedNodes),
					nodeGroup.Opts.MaxNodes,
				)
			}
		}
	}

	log.WithField("nodegroup", nodegroup).Debugln("DeltaScaled=", nodesDelta)

	// Perform the scaling action
	switch {
	case nodesDelta < 0:
		nodesToRemove := -nodesDelta
		log.WithField("nodegroup", nodegroup).Infof("Scaling Down: tainting %v nodes", nodesToRemove)
		metrics.NodeGroupTaintEvent.WithLabelValues(nodegroup).Add(float64(nodesToRemove))

		// Lock the taintinf to a maximum on 10 nodes
		if err := k8s.BeginTaintFailSafe(nodesToRemove); err != nil {
			// Don't taint if there was an error on the lock
			log.Errorf("Failed to get safetly lock on tainter: %v", err)
			break
		}
		// Perform the tainting loop with the fail safe around it
		tainted := c.taintOldestN(untaintedNodes, nodeGroup, nodesToRemove)
		// Validate the Failsafe worked
		if err := k8s.EndTaintFailSafe(len(tainted)); err != nil {
			log.Errorf("Failed to validate safetly lock on tainter: %v", err)
			break
		}

		log.Infof("Tainted a total of %v nodes", len(tainted))

	case nodesDelta > 0:
		nodesToAdd := nodesDelta
		if len(taintedNodes) == 0 {
			log.WithField("nodegroup", nodegroup).Warningln("There are no tainted nodes to untaint")
			break
		}

		// Metrics & Logs
		log.WithField("nodegroup", nodegroup).Infof("Scaling Up: Trying to untaint %v tainted nodes", nodesToAdd)
		metrics.NodeGroupUntaintEvent.WithLabelValues(nodegroup).Add(float64(nodesToAdd))

		untainted := c.untaintNewestN(taintedNodes, nodeGroup, nodesToAdd)
		log.Infof("Untainted a total of %v nodes", len(untainted))
	default:
		log.WithField("nodegroup", nodegroup).Infoln("No need to scale")
	}
}

// RunOnce performs the main autoscaler logic once
func (c Controller) RunOnce() {
	startTime := time.Now()

	// TODO(jgonzalez/dangot):
	// REAPER GOES HERE

	// Perform the ScaleUp/Taint logic
	for nodegroup, state := range c.nodeGroups {
		log.Debugln("**********[START NODEGROUP]**********")
		c.scaleNodeGroup(nodegroup, state)
	}

	endTime := time.Now()
	log.Debugf("Scaling took a total of %v", endTime.Sub(startTime))
}

// RunForever starts the autoscaler process and runs once every ScanInterval. blocks thread
func (c Controller) RunForever(runImmediately bool) {
	if runImmediately {
		log.Debugln("**********[AUTOSCALER FIRST LOOP]**********")
		c.RunOnce()
	}

	// Start the main loop
	ticker := time.NewTicker(c.Opts.ScanInterval)
	for {
		select {
		case <-ticker.C:
			log.Debugln("**********[AUTOSCALER MAIN LOOP]**********")
			c.RunOnce()
		case <-c.stopChan:
			log.Debugf("Stopping main loop")
			ticker.Stop()
			return
		}
	}
}
