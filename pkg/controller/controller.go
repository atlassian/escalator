package controller

import (
	"errors"
	"math"
	"time"

	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"

	"github.com/atlassian/escalator/pkg/cloudprovider"
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
	Opts     Opts
	stopChan <-chan struct{}

	cloudProvider cloudprovider.CloudProvider

	nodeGroups map[string]*NodeGroupState
}

// NodeGroupState contains everything about a node group in the current state of the application
type NodeGroupState struct {
	Opts NodeGroupOptions
	*NodeGroupLister

	NodeInfos map[string]*schedulercache.NodeInfo

	ASG cloudprovider.NodeGroup

	// used for tracking which nodes are tainted. testing when in dry mode
	taintTracker []string
}

// Opts provide the Controller with config for runtime
type Opts struct {
	K8SClient            kubernetes.Interface
	NodeGroups           []NodeGroupOptions
	CloudProviderBuilder cloudprovider.Builder

	ScanInterval time.Duration
	DryMode      bool
}

// scaleOpts provides options for a scale function
// wraps options that would be passed as args
type scaleOpts struct {
	nodes               []*v1.Node
	taintedNodes        []*v1.Node
	untaintedNodes      []*v1.Node
	pods                []*v1.Pod
	nodeGroup           *NodeGroupState
	clusterUsagePercent int
	nodesDelta          int
}

// NewController creates a new controller with the specified options
func NewController(opts Opts, stopChan <-chan struct{}) *Controller {
	client := NewClient(opts.K8SClient, opts.NodeGroups, stopChan)
	if client == nil {
		log.Fatalln("Failed to create controller client")
		return nil
	}

	cloud := opts.CloudProviderBuilder.Build()
	if cloud == nil {
		log.Fatal("Failed to create cloudprovider")
	}

	// turn it into a map of name and nodegroupstate for O(1) lookup and data bundling
	nodegroupMap := make(map[string]*NodeGroupState)
	for _, nodeGroupOpts := range opts.NodeGroups {
		asg, ok := cloud.GetNodeGroup(nodeGroupOpts.CloudProviderASG)
		if !ok {
			log.Fatalf("could not find asg nodegroup \"%v\" on cloudprovider", nodeGroupOpts.CloudProviderASG)
		}
		nodegroupMap[nodeGroupOpts.Name] = &NodeGroupState{
			Opts:            nodeGroupOpts,
			NodeGroupLister: client.Listers[nodeGroupOpts.Name],
			ASG:             asg,
		}
	}

	return &Controller{
		Client:        client,
		Opts:          opts,
		stopChan:      stopChan,
		cloudProvider: cloud,
		nodeGroups:    nodegroupMap,
	}
}

// dryMode is a helper that returns the overall drymode result of the controller and nodegroup
func (c *Controller) dryMode(nodeGroup *NodeGroupState) bool {
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

// filterNodes separates nodes between tainted and untainted nodes
func (c *Controller) filterNodes(nodeGroup *NodeGroupState, allNodes []*v1.Node) (untaintedNodes []*v1.Node, taintedNodes []*v1.Node) {
	untaintedNodes = make([]*v1.Node, 0, len(allNodes))
	taintedNodes = make([]*v1.Node, 0, len(allNodes))

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

	return
}

// scaleNodeGroup performs the core logic of calculating util and choosig a scaling action for a node group
func (c *Controller) scaleNodeGroup(nodegroup string, nodeGroup *NodeGroupState) {
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
	untaintedNodes, taintedNodes := c.filterNodes(nodeGroup, allNodes)

	// Metrics and Logs
	log.WithField("nodegroup", nodegroup).Infoln("nodes remaining total:", len(allNodes))
	log.WithField("nodegroup", nodegroup).Infoln("nodes remaining untainted:", len(untaintedNodes))
	log.WithField("nodegroup", nodegroup).Infoln("nodes remaining tainted:", len(taintedNodes))
	metrics.NodeGroupNodes.WithLabelValues(nodegroup).Set(float64(len(allNodes)))
	metrics.NodeGroupNodesUntainted.WithLabelValues(nodegroup).Set(float64(len(untaintedNodes)))
	metrics.NodeGroupNodesTainted.WithLabelValues(nodegroup).Set(float64(len(taintedNodes)))
	metrics.NodeGroupPods.WithLabelValues(nodegroup).Set(float64(len(pods)))

	// We want to be really simple right now so we don't do anything if we are outside the range of allowed nodes
	// We assume it is a config error or something bad has gone wrong in the cluster
	if len(allNodes) == 0 {
		log.WithField("nodegroup", nodegroup).Warningln("no nodes remaining")
		return
	}
	if len(allNodes) < nodeGroup.Opts.MinNodes {
		log.WithField("nodegroup", nodegroup).Warningf(
			"Node count of %v less than minimum of %v",
			len(allNodes),
			nodeGroup.Opts.MinNodes,
		)
		return
	}
	if len(allNodes) > nodeGroup.Opts.MaxNodes {
		log.WithField("nodegroup", nodegroup).Warningf(
			"Node count of %v larger than maximum of %v",
			len(allNodes),
			nodeGroup.Opts.MaxNodes,
		)
		return
	}

	// update the map of node to nodeinfo
	// for working out which pods are on which nodes
	nodeGroup.NodeInfos = k8s.CreateNodeNameToInfoMap(pods, allNodes)

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
	// Need to scale up so capacity can handle requests
	case maxPercent > nodeGroup.Opts.ScaleUpThreshholdPercent:
		// if ScaleUpThreshholdPercent is our "max target" or "slack capacity"
		// we want to add enough nodes such that the maxPercentage cluster util
		// drops back below ScaleUpThreshholdPercent

		// we assume that all nodes have the same capacity
		nodeWorth := 1.0 / float64(len(allNodes)) * 100.0

		percentageNeededCPU := cpuPercent - float64(nodeGroup.Opts.ScaleUpThreshholdPercent)
		percentageNeededMem := memPercent - float64(nodeGroup.Opts.ScaleUpThreshholdPercent)

		nodesNeededCPU := math.Ceil(percentageNeededCPU / nodeWorth)
		nodesNeededMem := math.Ceil(percentageNeededMem / nodeWorth)

		nodesDelta = int(math.Max(nodesNeededCPU, nodesNeededMem))
	}

	log.WithField("nodegroup", nodegroup).Debugln("Delta=", nodesDelta)

	scaleOptions := scaleOpts{
		nodes:               allNodes,
		taintedNodes:        taintedNodes,
		untaintedNodes:      untaintedNodes,
		pods:                pods,
		nodeGroup:           nodeGroup,
		clusterUsagePercent: maxPercent,
	}

	// Clamp the nodes inside the min and max node count
	var nodesDeltaResult int
	switch {
	case nodesDelta < 0:
		// Try to scale down
		scaleOptions.nodesDelta = -nodesDelta
		nodesDeltaResult, err = c.ScaleDown(scaleOptions)
		if err != nil {
			log.WithField("nodegroup", nodegroup).Error(err)
		}
	case nodesDelta > 0:
		// Try to scale up
		scaleOptions.nodesDelta = nodesDelta
		nodesDeltaResult, err = c.ScaleUp(scaleOptions)
		if err != nil {
			log.WithField("nodegroup", nodegroup).Error(err)
		}
	default:
		log.WithField("nodegroup", nodegroup).Infoln("No need to scale")
		// reap any expired nodes
		removed, err := c.TryRemoveTaintedNodes(scaleOptions)
		if err != nil {
			log.WithField("nodegroup", nodegroup).Error(err)
		}
		log.WithField("nodegroup", nodegroup).Infoln("Reaper: There were", removed, "empty nodes deleted this round")
	}

	log.WithField("nodegroup", nodegroup).Debugln("DeltaScaled=", nodesDeltaResult)
}

// RunOnce performs the main autoscaler logic once
func (c *Controller) RunOnce() {
	startTime := time.Now()

	c.cloudProvider.Refresh()
	// Perform the ScaleUp/Taint logic
	for nodegroup, state := range c.nodeGroups {
		log.Debugln("**********[START NODEGROUP]**********")
		c.scaleNodeGroup(nodegroup, state)
	}

	endTime := time.Now()
	log.Debugf("Scaling took a total of %v", endTime.Sub(startTime))
}

// RunForever starts the autoscaler process and runs once every ScanInterval. blocks thread
func (c *Controller) RunForever(runImmediately bool) {
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
