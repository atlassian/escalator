package controller

import (
	"math"
	"time"

	"github.com/atlassian/escalator/pkg/cloudprovider"
	"github.com/atlassian/escalator/pkg/k8s"
	"github.com/atlassian/escalator/pkg/metrics"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
)

// Controller contains the core logic of the Autoscaler
type Controller struct {
	Client        *Client
	Opts          Opts
	stopChan      <-chan struct{}
	cloudProvider cloudprovider.CloudProvider
	nodeGroups    map[string]*NodeGroupState
}

// NodeGroupState contains everything about a node group in the current state of the application
type NodeGroupState struct {
	*NodeGroupLister
	Opts        NodeGroupOptions
	NodeInfoMap map[string]*k8s.NodeInfo
	scaleUpLock scaleLock

	// used for tracking which nodes are tainted. testing when in dry mode
	taintTracker []string

	// used for tracking scale delta across runs, useful for reducing hysteresis
	scaleDelta   int
	lastScaleOut time.Time

	// used for storing cached instance capacity
	cpuCapacity resource.Quantity
	memCapacity resource.Quantity
}

// Opts provide the Controller with config for runtime
type Opts struct {
	K8SClient            kubernetes.Interface
	NodeGroups           []NodeGroupOptions
	CloudProviderBuilder cloudprovider.Builder
	ScanInterval         time.Duration
	DryMode              bool
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
func NewController(opts Opts, stopChan <-chan struct{}) (*Controller, error) {
	client, err := NewClient(opts.K8SClient, opts.NodeGroups, stopChan)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create controller client")
	}

	cloud, err := opts.CloudProviderBuilder.Build()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create cloudprovider")
	}

	// turn it into a map of name and nodegroupstate for O(1) lookup and data bundling
	nodegroupMap := make(map[string]*NodeGroupState)
	for _, nodeGroupOpts := range opts.NodeGroups {
		cloudProviderNodeGroup, ok := cloud.GetNodeGroup(nodeGroupOpts.CloudProviderGroupName)
		if !ok {
			return nil, errors.Errorf("could not find node group \"%v\" on cloud provider", nodeGroupOpts.CloudProviderGroupName)
		}

		// Set the node group min_nodes and max_nodes options based on the values in the cloud provider
		if nodeGroupOpts.autoDiscoverMinMaxNodeOptions() {
			nodeGroupOpts.MinNodes = int(cloudProviderNodeGroup.MinSize())
			log.Debugf("auto discovered min_nodes = %v for node group %v", nodeGroupOpts.MinNodes, nodeGroupOpts.Name)
			nodeGroupOpts.MaxNodes = int(cloudProviderNodeGroup.MaxSize())
			log.Debugf("auto discovered max_nodes = %v for node group %v", nodeGroupOpts.MaxNodes, nodeGroupOpts.Name)
		}

		nodegroupMap[nodeGroupOpts.Name] = &NodeGroupState{
			Opts:            nodeGroupOpts,
			NodeGroupLister: client.Listers[nodeGroupOpts.Name],
			// Setup the scaleLock timeouts for this nodegroup
			scaleUpLock: scaleLock{
				minimumLockDuration: nodeGroupOpts.ScaleUpCoolDownPeriodDuration(),
				nodegroup:           nodeGroupOpts.Name,
			},
			scaleDelta: 0,
		}
	}

	return &Controller{
		Client:        client,
		Opts:          opts,
		stopChan:      stopChan,
		cloudProvider: cloud,
		nodeGroups:    nodegroupMap,
	}, nil
}

// dryMode is a helper that returns the overall drymode result of the controller and nodegroup
func (c *Controller) dryMode(nodeGroup *NodeGroupState) bool {
	return c.Opts.DryMode || nodeGroup.Opts.DryMode
}

// filterNodes separates nodes between tainted and untainted nodes
func (c *Controller) filterNodes(nodeGroup *NodeGroupState, allNodes []*v1.Node) (untaintedNodes, taintedNodes, cordonedNodes []*v1.Node) {
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

// calculateNewNodeMetrics checks if there are new nodes and calculates metrics
func (c *Controller) calculateNewNodeMetrics(nodegroup string, nodeGroup *NodeGroupState) {
	// If we are not locked, we're either init or after a scale event
	// If the last scale event was a scale out; i.e. nodesDelta > 0
	// Calculate the k8s registration latency from cloud provider
	// resource instantiation

	// Concerned about:
	// - lock may open before new nodes register and they will be missed
	if nodeGroup.scaleDelta > 0 {
		countNewNodes := 0

		for key, nodeInfo := range nodeGroup.NodeInfoMap {
			nodeRegTime := nodeInfo.Node().ObjectMeta.CreationTimestamp.Time
			// Check if node registration time newer than last scale out
			if nodeRegTime.Sub(nodeGroup.lastScaleOut) > 0 {
				node := nodeInfo.Node()
				instance, err := c.cloudProvider.GetInstance(node)
				if err != nil {
					log.Error("Unable to get instance from cloud provider to determine registration lag, skipping ", node.Spec.ProviderID)
				} else {
					nodeRegistrationLag := nodeRegTime.Sub(instance.InstantiationTime())
					log.Debugf("Delta between node instantiation time and node registration: %v - %v", key, nodeRegistrationLag)
					metrics.NodeGroupNodeRegistrationLag.WithLabelValues(nodegroup).Observe(nodeRegistrationLag.Seconds())
					countNewNodes++
				}
			}
		}

		if countNewNodes != nodeGroup.scaleDelta {
			log.Warningf("Expected new nodes: %v Actual new nodes: %v", nodeGroup.scaleDelta, countNewNodes)
		}
	}
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

	// store a cached version of node capacity
	if len(allNodes) > 0 {
		nodeGroup.cpuCapacity = *allNodes[0].Status.Allocatable.Cpu()
		nodeGroup.memCapacity = *allNodes[0].Status.Allocatable.Memory()
	}

	// Filter into untainted and tainted nodes
	untaintedNodes, taintedNodes, cordonedNodes := c.filterNodes(nodeGroup, allNodes)

	// Metrics and Logs
	log.WithField("nodegroup", nodegroup).Infof("pods total: %v", len(pods))
	log.WithField("nodegroup", nodegroup).Infof("nodes remaining total: %v", len(allNodes))
	log.WithField("nodegroup", nodegroup).Infof("cordoned nodes remaining total: %v", len(cordonedNodes))
	log.WithField("nodegroup", nodegroup).Infof("nodes remaining untainted: %v", len(untaintedNodes))
	log.WithField("nodegroup", nodegroup).Infof("nodes remaining tainted: %v", len(taintedNodes))
	log.WithField("nodegroup", nodegroup).Infof("Minimum Node: %v", nodeGroup.Opts.MinNodes)
	log.WithField("nodegroup", nodegroup).Infof("Maximum Node: %v", nodeGroup.Opts.MaxNodes)
	metrics.NodeGroupNodes.WithLabelValues(nodegroup).Set(float64(len(allNodes)))
	metrics.NodeGroupNodesCordoned.WithLabelValues(nodegroup).Set(float64(len(cordonedNodes)))
	metrics.NodeGroupNodesUntainted.WithLabelValues(nodegroup).Set(float64(len(untaintedNodes)))
	metrics.NodeGroupNodesTainted.WithLabelValues(nodegroup).Set(float64(len(taintedNodes)))
	metrics.NodeGroupPods.WithLabelValues(nodegroup).Set(float64(len(pods)))

	// We want to be really simple right now so we don't do anything if we are outside the range of allowed nodes
	// We assume it is a config error or something bad has gone wrong in the cluster

	if len(allNodes) == 0 && len(pods) == 0 {
		log.WithField("nodegroup", nodegroup).Info("no pods requests and remain 0 node for node group")
		return 0, nil
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
			nodes:          allNodes,
			nodesDelta:     nodeGroup.Opts.MinNodes - len(untaintedNodes),
			nodeGroup:      nodeGroup,
			taintedNodes:   taintedNodes,
			untaintedNodes: untaintedNodes,
		})
		if err != nil {
			log.WithField("nodegroup", nodegroup).Error(err)
		}
		return result, err
	}

	// Calc %
	// both cpu and memory capacity are based on number of untainted nodes
	// pass number of untainted nodes in to help make decision if it's a scaling-up-from-0
	cpuPercent, memPercent, err := calcPercentUsage(cpuRequest, memRequest, cpuCapacity, memCapacity, int64(len(untaintedNodes)))
	if err != nil {
		log.Errorf("Failed to calculate percentages: %v", err)
		return 0, err
	}

	// Metrics
	log.WithField("nodegroup", nodegroup).Infof("cpu: %v, memory: %v", cpuPercent, memPercent)

	// on the case that we're scaling up from 0, emit 0 as the metrics to keep metrics sane
	if cpuPercent == math.MaxFloat64 || memPercent == math.MaxFloat64 {
		metrics.NodeGroupsCPUPercent.WithLabelValues(nodegroup).Set(0)
		metrics.NodeGroupsMemPercent.WithLabelValues(nodegroup).Set(0)
	} else {
		metrics.NodeGroupsCPUPercent.WithLabelValues(nodegroup).Set(cpuPercent)
		metrics.NodeGroupsMemPercent.WithLabelValues(nodegroup).Set(memPercent)
	}

	locked := nodeGroup.scaleUpLock.locked()
	if locked {
		// don't do anything else until we're unlocked again
		log.WithField("nodegroup", nodegroup).Info(nodeGroup.scaleUpLock)
		log.WithField("nodegroup", nodegroup).Info("Waiting for scale to finish")
		return nodeGroup.scaleUpLock.requestedNodes, nil
	}

	c.calculateNewNodeMetrics(nodegroup, nodeGroup)

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
		nodesDelta, err = calcScaleUpDelta(untaintedNodes, cpuPercent, memPercent, cpuRequest, memRequest, nodeGroup)
		if err != nil {
			log.Errorf("Failed to calculate node delta: %v", err)
			return nodesDelta, err
		}
	}

	log.WithField("nodegroup", nodegroup).Debugf("Delta: %v", nodesDelta)

	scaleOptions := scaleOpts{
		nodes:          allNodes,
		taintedNodes:   taintedNodes,
		untaintedNodes: untaintedNodes,
		nodeGroup:      nodeGroup,
	}

	// Perform a scale up, do nothing or scale down based on the nodes delta
	var nodesDeltaResult int
	// actionErr keeps the error of any action below and checked after action
	// make sure shadowing variable won't be created for it
	var actionErr error
	switch {
	case nodesDelta < 0:
		// Try to scale down
		scaleOptions.nodesDelta = -nodesDelta
		nodesDeltaResult, actionErr = c.ScaleDown(scaleOptions)
	case nodesDelta > 0:
		// Try to scale up
		scaleOptions.nodesDelta = nodesDelta
		nodesDeltaResult, actionErr = c.ScaleUp(scaleOptions)
		nodeGroup.lastScaleOut = time.Now()
	default:
		log.WithField("nodegroup", nodegroup).Info("No need to scale")
		// reap any expired nodes
		var removed int
		removed, actionErr = c.TryRemoveTaintedNodes(scaleOptions)
		log.WithField("nodegroup", nodegroup).Infof("Reaper: There were %v empty nodes deleted this round", removed)
	}

	if actionErr != nil {
		switch actionErr.(type) {
		// early return when node is NOT in expected node group
		case *cloudprovider.NodeNotInNodeGroup:
			return 0, actionErr
		default:
			log.WithField("nodegroup", nodegroup).Error(actionErr)
		}
	}

	log.WithField("nodegroup", nodegroup).Debugf("DeltaScaled: %v", nodesDeltaResult)
	return nodesDelta, err
}

// RunOnce performs the main autoscaler logic once
func (c *Controller) RunOnce() error {
	startTime := time.Now()

	// try refresh cred a few times if they go stale
	// rebuild will create a new session from the metadata on the box
	err := c.cloudProvider.Refresh()
	for i := 0; i < 2 && err != nil; i++ {
		log.Warnf("cloud provider failed to refresh. trying to re-fetch credentials. tries = %v", i+1)
		time.Sleep(5 * time.Second) // sleep to allow kube2iam to fill node with metadata
		c.cloudProvider, err = c.Opts.CloudProviderBuilder.Build()
		if err != nil {
			return err
		}
		err = c.cloudProvider.Refresh()
	}
	// Perform the ScaleUp/Taint logic
	for _, nodeGroupOpts := range c.Opts.NodeGroups {
		log.Debugf("**********[START NODEGROUP %v]**********", nodeGroupOpts.Name)
		state := c.nodeGroups[nodeGroupOpts.Name]
		// Double check if node group still exists from the cloud provider then retrieve the latest stat
		cloudProviderNodeGroup, ok := c.cloudProvider.GetNodeGroup(nodeGroupOpts.CloudProviderGroupName)
		if !ok {
			err = errors.New("could not find node group")
			return err
		}
		// Update the min_nodes and max_nodes based on the latest value from the cloud provider
		if nodeGroupOpts.autoDiscoverMinMaxNodeOptions() {
			state.Opts.MinNodes = int(cloudProviderNodeGroup.MinSize())
			log.Debugf("auto discovered min_nodes = %v for node group %v", state.Opts.MinNodes, nodeGroupOpts.Name)
			state.Opts.MaxNodes = int(cloudProviderNodeGroup.MaxSize())
			log.Debugf("auto discovered max_nodes = %v for node group %v", state.Opts.MaxNodes, nodeGroupOpts.Name)
		}
		delta, err := c.scaleNodeGroup(nodeGroupOpts.Name, state)
		metrics.NodeGroupScaleDelta.WithLabelValues(nodeGroupOpts.Name).Set(float64(delta))
		state.scaleDelta = delta
		if err != nil {
			switch err.(type) {
			// return error which will cause app erroring out
			case *cloudprovider.NodeNotInNodeGroup:
				return err
			default:
				log.Warn(err)
			}

		}
	}

	metrics.RunCount.Add(1)
	endTime := time.Now()
	log.Debugf("Scaling took a total of %v", endTime.Sub(startTime))
	return nil
}

// RunForever starts the autoscaler process and runs once every ScanInterval. blocks thread
// it always returns a non-nil error
func (c *Controller) RunForever(runImmediately bool) error {
	if runImmediately {
		log.Debug("**********[AUTOSCALER FIRST LOOP]**********")
		err := c.RunOnce()
		if err != nil {
			return err
		}
	}

	// Start the main loop
	ticker := time.NewTicker(c.Opts.ScanInterval)
	for {
		select {
		case <-ticker.C:
			log.Debug("**********[AUTOSCALER MAIN LOOP]**********")
			err := c.RunOnce()
			if err != nil {
				return err
			}
		case <-c.stopChan:
			log.Debugf("Stopping main loop")
			ticker.Stop()
			return errors.New("main loop stopped")
		}
	}
}
