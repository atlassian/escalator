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
	*Client
	*Opts
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
	K8SClient kubernetes.Interface
	Customers []*NodeGroupOptions

	ScanInterval time.Duration
	DryMode      bool
}

// NewController creates a new controller with the specified options
func NewController(opts *Opts, stopChan <-chan struct{}) *Controller {
	client := NewClient(opts.K8SClient, opts.Customers, stopChan)
	if client == nil {
		log.Fatalln("Failed to create controller client")
		return nil
	}

	// turn it into a map of name and nodegroupstate for O(1) lookup and data bundling
	customerMap := make(map[string]*NodeGroupState)
	for _, nodeGroupOpts := range opts.Customers {
		customerMap[nodeGroupOpts.Name] = &NodeGroupState{
			Opts:            nodeGroupOpts,
			NodeGroupLister: client.Listers[nodeGroupOpts.Name],
		}
	}

	return &Controller{
		Client:     client,
		Opts:       opts,
		stopChan:   stopChan,
		nodeGroups: customerMap,
	}
}

// DryMode is a helper that returns the overall drymode result of the controller and nodegroup
func (c Controller) DryMode(nodeGroup *NodeGroupState) bool {
	return c.Opts.DryMode || nodeGroup.Opts.DryMode
}

func calcPercentUsage(cpuR, memR, cpuA, memA resource.Quantity) (float64, float64, error) {
	if cpuA.MilliValue() == 0 || memA.MilliValue() == 0 {
		return 0, 0, errors.New("Cannot divide by zero in percent calculation")
	}
	cpuPercent := float64(cpuR.MilliValue()) / float64(cpuA.MilliValue()) * 100
	memPercent := float64(memR.MilliValue()) / float64(memA.MilliValue()) * 100
	return cpuPercent, memPercent, nil
}

func doesItFit(cpuR, memR, cpuA, memA resource.Quantity) (bool, error) {
	if cpuR.Cmp(cpuA) == -1 {
		log.Infof("There is enough CPU")
	}

	if memR.Cmp(memA) == -1 {
		log.Infof("There is enough Memory")
	}

	return true, nil
}

// taintOldestN sorts nodes by creation time and taints the oldest N. It will return an array of indecies of the nodes it tainted
// indices are from the parameter nodes indexes, not the sorted index
func (c Controller) taintOldestN(nodes []*v1.Node, nodeGroup *NodeGroupState, n int) []int {
	sorted := make(nodesByOldestCreationTime, 0, len(nodes))
	for i, node := range nodes {
		sorted = append(sorted, nodeIndexBundle{node, i})
	}
	sort.Sort(sorted)

	taintedIndices := make([]int, 0, n)
	for _, bundle := range sorted {
		// stop at N (or when array is fully iterated)
		if len(taintedIndices) >= n {
			break
		}

		// only actually taint in dry mode
		if !c.DryMode(nodeGroup) {
			log.WithField("drymode", "off").Infoln("Tainting node", bundle.node.Name)
			updatedNode, err := k8s.AddToBeRemovedTaint(bundle.node, c.Client)
			if err != nil {
				bundle.node = updatedNode
				taintedIndices = append(taintedIndices, bundle.index)
			}
		} else {
			nodeGroup.taintTracker = append(nodeGroup.taintTracker, bundle.node.Name)
			taintedIndices = append(taintedIndices, bundle.index)
			log.WithField("drymode", "on").Infoln("Tainting node", bundle.node.Name)
		}
	}

	return taintedIndices
}

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
		if !c.DryMode(nodeGroup) {
			if _, tainted := k8s.GetToBeRemovedTaint(bundle.node); tainted {
				log.WithField("drymode", "off").Infoln("Untainting node", bundle.node.Name)
				updatedNode, err := k8s.DeleteToBeRemovedTaint(bundle.node, c.Client)
				if err != nil {
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

func (c Controller) scaleNodeGroup(customer string, nodeGroup *NodeGroupState) {
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

	// Filter out tainted nodes
	untaintedNodes := make([]*v1.Node, 0, len(allNodes))
	for _, node := range allNodes {
		if c.DryMode(nodeGroup) {
			var contains bool
			for _, name := range nodeGroup.taintTracker {
				if node.Name == name {
					contains = true
					break
				}
			}
			if !contains {
				untaintedNodes = append(untaintedNodes, node)
			}
		} else {
			if _, tainted := k8s.GetToBeRemovedTaint(node); !tainted {
				untaintedNodes = append(untaintedNodes, node)
			}
		}
	}
	log.WithField("customer", customer).Infoln("total nodes remaining:", len(allNodes))
	log.WithField("customer", customer).Infoln("nodes remaining not tainted:", len(untaintedNodes))

	if len(allNodes) == 0 {
		log.WithField("customer", customer).Infoln("no nodes remaining")
		return
	}

	// Metrics
	metrics.NodeGroupNodes.WithLabelValues(customer).Set(float64(len(untaintedNodes)))
	metrics.NodeGroupPods.WithLabelValues(customer).Set(float64(len(pods)))

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
	metrics.NodeGroupCPURequest.WithLabelValues(customer).Set(float64(cpuRequest.MilliValue()))
	bytesMemReq, _ := memRequest.AsInt64()
	metrics.NodeGroupMemRequest.WithLabelValues(customer).Set(float64(bytesMemReq))
	metrics.NodeGroupCPUCapacity.WithLabelValues(customer).Set(float64(cpuCapacity.MilliValue()))
	bytesMemCap, _ := memCapacity.AsInt64()
	metrics.NodeGroupMemCapacity.WithLabelValues(customer).Set(float64(bytesMemCap))

	// Calc %
	cpuPercent, memPercent, err := calcPercentUsage(cpuRequest, memRequest, cpuCapacity, memCapacity)
	if err != nil {
		log.Errorf("Failed to calculate percentages: %v", err)
		return
	}

	// Metrics
	log.WithField("customer", customer).Infof("cpu: %v, memory: %v", cpuPercent, memPercent)
	metrics.NodeGroupsCPUPercent.WithLabelValues(customer).Set(cpuPercent)
	metrics.NodeGroupsMemPercent.WithLabelValues(customer).Set(memPercent)

	// Scale down upper percentage threshhold
	if math.Max(cpuPercent, memPercent) < float64(nodeGroup.Opts.TaintUpperCapacityThreshholdPercent) && len(untaintedNodes) > nodeGroup.Opts.MinNodes {
		log.Warningln("Upper threshhold reached. Scale down 1 node")
		metrics.NodeGroupTaintEvent.WithLabelValues(customer).Inc()
		nodesTainted := c.taintOldestN(untaintedNodes, nodeGroup, 1)

		// remove all the nodes we just tainted from further calculations
		filtered := make([]*v1.Node, 0, len(untaintedNodes)-1)
		for i, node := range untaintedNodes {
			for _, j := range nodesTainted {
				if i != j {
					filtered = append(filtered, node)
				}
			}
		}

		// Scale down lower percentage threshhold
		if math.Max(cpuPercent, memPercent) < float64(nodeGroup.Opts.TaintLowerCapacityThreshholdPercent) && len(filtered)-2 > nodeGroup.Opts.MinNodes {
			log.Warningln("Lower threshhold reached. Scale down 1 node")
			metrics.NodeGroupTaintEvent.WithLabelValues(customer).Inc()
			c.taintOldestN(filtered, nodeGroup, 2)
		}
	}

	// Scale up by upper threshhold
	if math.Max(cpuPercent, memPercent) > float64(nodeGroup.Opts.UntaintLowerCapacityThreshholdPercent) && len(untaintedNodes)+1 < nodeGroup.Opts.MaxNodes {
		log.Warningln("Slack space reached. Trying to untaint a tainted node")
		metrics.NodeGroupTaintEvent.WithLabelValues(customer).Dec()
		// push in allNodes here to include tainted ones. Otherwise it will untaint nothing
		nodesUntainted := c.untaintNewestN(allNodes, nodeGroup, 1)

		// remove all the nodes we just untainted from further calculations
		filtered := make([]*v1.Node, 0, len(allNodes)-1)
		for i, node := range untaintedNodes {
			for _, j := range nodesUntainted {
				if i != j {
					filtered = append(filtered, node)
				}
			}
		}

		// Scale down lower percentage threshhold
		if math.Max(cpuPercent, memPercent) > float64(nodeGroup.Opts.UntaintUpperCapacityThreshholdPercent) && len(filtered)+2 > nodeGroup.Opts.MinNodes {
			log.Warningln("Agressive Slack space reached. Trying to untaint tainted nodes")
			metrics.NodeGroupTaintEvent.WithLabelValues(customer).Inc()
			c.untaintNewestN(filtered, nodeGroup, 2)
		}
	}
}

// RunOnce performs the main autoscaler logic once
func (c Controller) RunOnce() {
	startTime := time.Now()

	// TODO(jgonzalez/dangot):
	// REAPER GOES HERE

	// Perform the ScaleUp/Taint logic
	for customer, state := range c.nodeGroups {
		c.scaleNodeGroup(customer, state)
	}

	endTime := time.Now()
	log.Infof("Scaling took a total of %v", endTime.Sub(startTime))
}

// RunForever starts the autoscaler process and runs once every ScanInterval. blocks thread
func (c Controller) RunForever(runImmediately bool) {
	if runImmediately {
		log.Infoln("---[FIRSTRUN LOOP]---")
		c.RunOnce()
	}

	// Start the main loop
	ticker := time.NewTicker(c.Opts.ScanInterval)
	for {
		select {
		case <-ticker.C:
			log.Infoln("---[AUTOSCALER LOOP]---")
			c.RunOnce()
		case <-c.stopChan:
			log.Debugf("Stopping main loop")
			ticker.Stop()
			return
		}
	}
}
