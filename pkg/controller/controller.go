package controller

import (
	"math"
	"time"

	"k8s.io/kubernetes/pkg/scheduler/cache"

	"github.com/atlassian/escalator/pkg/cloudprovider"
	"github.com/atlassian/escalator/pkg/k8s"
	"github.com/atlassian/escalator/pkg/metrics"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"errors"

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

	NodeInfoMap map[string]*cache.NodeInfo

	scaleUpLock scaleLock

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
	nodes          []*v1.Node
	taintedNodes   []*v1.Node
	untaintedNodes []*v1.Node
	nodeGroup      *NodeGroupState
	nodesDelta     int
}

// NewController creates a new controller with the specified options
func NewController(opts Opts, stopChan <-chan struct{}) *Controller {
	client := NewClient(opts.K8SClient, opts.NodeGroups, stopChan)
	if client == nil {
		log.Fatal("Failed to create controller client")
		return nil
	}

	cloud, err := opts.CloudProviderBuilder.Build()
	if err != nil {
		log.Fatal("Failed to create cloudprovider", err)
	}

	// turn it into a map of name and nodegroupstate for O(1) lookup and data bundling
	nodegroupMap := make(map[string]*NodeGroupState)
	for _, nodeGroupOpts := range opts.NodeGroups {
		cloudProviderNodeGroup, ok := cloud.GetNodeGroup(nodeGroupOpts.CloudProviderGroupName)
		if !ok {
			log.Fatalf("could not find node group \"%v\" on cloud provider", nodeGroupOpts.CloudProviderGroupName)
		}

		// Set the node group min_nodes and max_nodes options based on the values in the cloud provider
		if nodeGroupOpts.autoDiscoverMinMaxNodeOptions() {
			nodeGroupOpts.MinNodes = int(cloudProviderNodeGroup.MinSize())
			log.Debugf("auto discovered min_nodes = %v for node group %v", nodeGroupOpts.MinNodes, nodeGroupOpts.Name)
			nodeGroupOpts.MaxNodes = int(cloudProviderNodeGroup.MaxSize())
			log.Debugf("auto discovered max_nodes = %v for node group %v", nodeGroupOpts.MaxNodes, nodeGroupOpts.Name)
		}

		nodegroupMap[nodeGroupOpts.Name] = &NodeGroupState{
			Opts:                   nodeGroupOpts,
			NodeGroupLister:        client.Listers[nodeGroupOpts.Name],
			// Setup the scaleLock timeouts for this nodegroup
			scaleUpLock: scaleLock{
				minimumLockDuration: nodeGroupOpts.ScaleUpCoolDownPeriodDuration(),
				nodegroup: nodeGroupOpts.Name,
			},
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

// filterNodes separates nodes between tainted and untainted nodes
func (c *Controller) filterNodes(nodeGroup *NodeGroupState, allNodes []*v1.Node) (untaintedNodes []*v1.Node, taintedNodes []*v1.Node, cordonedNodes []*v1.Node) {
	untaintedNodes = make([]*v1.Node, 0, len(allNodes))
	taintedNodes = make([]*v1.Node, 0, len(allNodes))
	cordonedNodes = make([]*v1.Node, 0, len(allNodes))

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
			// If the node is Unschedulable (cordoned), separate it out from the tainted/untainted
			if node.Spec.Unschedulable {
				cordonedNodes = append(cordonedNodes, node)
				continue
			}
			if _, tainted := k8s.GetToBeRemovedTaint(node); !tainted {
				untaintedNodes = append(untaintedNodes, node)
			} else {
				taintedNodes = append(taintedNodes, node)
			}
		}
	}

	return
}

// scaleNodeGroup performs the core logic of calculating util and selecting a scaling action for a node group
func (c *Controller) scaleNodeGroup(nodegroup string, nodeGroup *NodeGroupState) (int, error) {
	// list all pods
	pods, err := nodeGroup.Pods.List()
	if err != nil {
		log.Errorf("Failed to list pods: %v", err)
		return 0, err
	}

	// List all nodes
	allNodes, err := nodeGroup.Nodes.List()
	if err != nil {
		log.Errorf("Failed to list nodes: %v", err)
		return 0, err
	}

	// Filter into untainted and tainted nodes
	untaintedNodes, taintedNodes, cordonedNodes := c.filterNodes(nodeGroup, allNodes)

	// Metrics and Logs
	log.WithField("nodegroup", nodegroup).Infof("pods total: %v", len(pods))
	log.WithField("nodegroup", nodegroup).Infof("nodes remaining total: %v", len(allNodes))
	log.WithField("nodegroup", nodegroup).Infof("cordoned nodes remaining total: %v", len(cordonedNodes))
	log.WithField("nodegroup", nodegroup).Infof("nodes remaining untainted: %v", len(untaintedNodes))
	log.WithField("nodegroup", nodegroup).Infof("nodes remaining tainted: %v", len(taintedNodes))
	metrics.NodeGroupNodes.WithLabelValues(nodegroup).Set(float64(len(allNodes)))
	metrics.NodeGroupNodesCordoned.WithLabelValues(nodegroup).Set(float64(len(cordonedNodes)))
	metrics.NodeGroupNodesUntainted.WithLabelValues(nodegroup).Set(float64(len(untaintedNodes)))
	metrics.NodeGroupNodesTainted.WithLabelValues(nodegroup).Set(float64(len(taintedNodes)))
	metrics.NodeGroupPods.WithLabelValues(nodegroup).Set(float64(len(pods)))

	// We want to be really simple right now so we don't do anything if we are outside the range of allowed nodes
	// We assume it is a config error or something bad has gone wrong in the cluster
	if len(allNodes) == 0 {
		err = errors.New("no nodes remaining")
		log.WithField("nodegroup", nodegroup).Warning(err.Error())
		return 0, err
	}
	if len(allNodes) < nodeGroup.Opts.MinNodes {
		err = errors.New("node count less than the minimum")
		log.WithField("nodegroup", nodegroup).Warningf(
			"Node count of %v less than minimum of %v",
			len(allNodes),
			nodeGroup.Opts.MinNodes,
		)
		return 0, err
	}
	if len(allNodes) > nodeGroup.Opts.MaxNodes {
		err = errors.New("node count larger than the maximum")
		log.WithField("nodegroup", nodegroup).Warningf(
			"Node count of %v larger than maximum of %v",
			len(allNodes),
			nodeGroup.Opts.MaxNodes,
		)
		return 0, err
	}

	// update the map of node to nodeinfo
	// for working out which pods are on which nodes
	nodeGroup.NodeInfoMap = k8s.CreateNodeNameToInfoMap(pods, allNodes)

	// Calc capacity for untainted nodes
	memRequest, cpuRequest, err := k8s.CalculatePodsRequestsTotal(pods)
	if err != nil {
		log.Errorf("Failed to calculate requests: %v", err)
		return 0, err
	}
	memCapacity, cpuCapacity, err := k8s.CalculateNodesCapacityTotal(untaintedNodes)
	if err != nil {
		log.Errorf("Failed to calculate capacity: %v", err)
		return 0, err
	}

	// Metrics
	metrics.NodeGroupCPURequest.WithLabelValues(nodegroup).Set(float64(cpuRequest.MilliValue()))
	metrics.NodeGroupCPUCapacity.WithLabelValues(nodegroup).Set(float64(cpuCapacity.MilliValue()))
	metrics.NodeGroupMemCapacity.WithLabelValues(nodegroup).Set(float64(memCapacity.MilliValue() / 1000))
	metrics.NodeGroupMemRequest.WithLabelValues(nodegroup).Set(float64(memRequest.MilliValue() / 1000))

	// If we ever get into a state where we have less nodes than the minimum
	if len(untaintedNodes) < nodeGroup.Opts.MinNodes {
		log.WithField("nodegroup", nodegroup).Warn("There are less untainted nodes than the minimum")
		result, err := c.ScaleUp(scaleOpts{
			nodes:      allNodes,
			nodesDelta: nodeGroup.Opts.MinNodes - len(untaintedNodes),
			nodeGroup:  nodeGroup,
		})
		if err != nil {
			log.WithField("nodegroup", nodegroup).Error(err)
		}
		return result, err
	}

	// Calc %
	cpuPercent, memPercent, err := calcPercentUsage(cpuRequest, memRequest, cpuCapacity, memCapacity)
	if err != nil {
		log.Errorf("Failed to calculate percentages: %v", err)
		return 0, err
	}

	// Metrics
	log.WithField("nodegroup", nodegroup).Infof("cpu: %v, memory: %v", cpuPercent, memPercent)
	metrics.NodeGroupsCPUPercent.WithLabelValues(nodegroup).Set(cpuPercent)
	metrics.NodeGroupsMemPercent.WithLabelValues(nodegroup).Set(memPercent)

	locked := nodeGroup.scaleUpLock.locked()
	lockVal := 0.0
	if locked {
		// don't do anything else until we're unlocked again
		lockVal = 1.0
		log.WithField("nodegroup", nodegroup).Info(nodeGroup.scaleUpLock)
		log.WithField("nodegroup", nodegroup).Info("Waiting for scale to finish")
		return nodeGroup.scaleUpLock.requestedNodes, nil
	}
	metrics.NodeGroupScaleLock.WithLabelValues(nodegroup).Observe(lockVal)

	// Perform the scaling decision
	maxPercent := math.Max(cpuPercent, memPercent)
	nodesDelta := 0

	// Determine if we want to scale up or down. Selects the first condition that is true
	switch {
	// --- Scale Down conditions ---
	// reached very low %. aggressively remove nodes
	case maxPercent < float64(nodeGroup.Opts.TaintLowerCapacityThresholdPercent):
		nodesDelta = -nodeGroup.Opts.FastNodeRemovalRate
	// reached medium low %. slowly remove nodes
	case maxPercent < float64(nodeGroup.Opts.TaintUpperCapacityThresholdPercent):
		nodesDelta = -nodeGroup.Opts.SlowNodeRemovalRate
	// --- Scale Up conditions ---
	// Need to scale up so capacity can handle requests
	case maxPercent > float64(nodeGroup.Opts.ScaleUpThresholdPercent):
		// if ScaleUpThresholdPercent is our "max target" or "slack capacity"
		// we want to add enough nodes such that the maxPercentage cluster util
		// drops back below ScaleUpThresholdPercent
		nodesDelta, err = calcScaleUpDelta(untaintedNodes, cpuPercent, memPercent, nodeGroup)
		if err != nil {
			log.Errorf("Failed to calculate node delta: %v", err)
			return nodesDelta, err
		}
	}

	log.WithField("nodegroup", nodegroup).Debugf("Delta: %v", nodesDelta)

	var nodesDeltaResult int
	scaleOptions := scaleOpts{
		nodes:          allNodes,
		taintedNodes:   taintedNodes,
		untaintedNodes: untaintedNodes,
		nodeGroup:      nodeGroup,
	}

	// Perform a scale up, do nothing or scale down based on the nodes delta
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
		log.WithField("nodegroup", nodegroup).Info("No need to scale")
		// reap any expired nodes
		removed, err := c.TryRemoveTaintedNodes(scaleOptions)
		if err != nil {
			log.WithField("nodegroup", nodegroup).Error(err)
		}
		log.WithField("nodegroup", nodegroup).Infof("Reaper: There were %v empty nodes deleted this round", removed)
	}

	log.WithField("nodegroup", nodegroup).Debugf("DeltaScaled: %v", nodesDeltaResult)
	return nodesDelta, err
}

// RunOnce performs the main autoscaler logic once
func (c *Controller) RunOnce() {
	startTime := time.Now()

	// try refresh cred a few times if they go stale
	// rebuild will create a new session from the metadata on the box
	err := c.cloudProvider.Refresh()
	for i := 0; i < 2 && err != nil; i++ {
		log.Warnf("cloud provider failed to refresh. trying to re-fetch credentials. tries = %v", i+1)
		time.Sleep(5 * time.Second) // sleep to allow kube2iam to fill node with metadata
		c.cloudProvider, err = c.Opts.CloudProviderBuilder.Build()
		if err != nil {
			log.Fatal(err)
		}
		err = c.cloudProvider.Refresh()
	}
	// Perform the ScaleUp/Taint logic
	for _, nodeGroupOpts := range c.Opts.NodeGroups {
		log.Debugf("**********[START NODEGROUP %v]**********", nodeGroupOpts.Name)
		state := c.nodeGroups[nodeGroupOpts.Name]
		delta, err := c.scaleNodeGroup(nodeGroupOpts.Name, state)
		metrics.NodeGroupScaleDelta.WithLabelValues(nodeGroupOpts.Name).Set(float64(delta))
		if err != nil {
			log.Warn(err)
		}
	}

	metrics.RunCount.Add(1)
	endTime := time.Now()
	log.Debugf("Scaling took a total of %v", endTime.Sub(startTime))
}

// RunForever starts the autoscaler process and runs once every ScanInterval. blocks thread
func (c *Controller) RunForever(runImmediately bool) {
	if runImmediately {
		log.Debug("**********[AUTOSCALER FIRST LOOP]**********")
		c.RunOnce()
	}

	// Start the main loop
	ticker := time.NewTicker(c.Opts.ScanInterval)
	for {
		select {
		case <-ticker.C:
			log.Debug("**********[AUTOSCALER MAIN LOOP]**********")
			c.RunOnce()
		case <-c.stopChan:
			log.Debugf("Stopping main loop")
			ticker.Stop()
			return
		}
	}
}
