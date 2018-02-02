package controller

import (
	"errors"
	"fmt"
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

	// used for tracking which nodes are tainted. testing when in dry mode
	taintTracker map[string][]string
}

// Opts provide the Controller with config for runtime
type Opts struct {
	K8SClient kubernetes.Interface
	Customers map[string]*NodeGroup

	ScanInterval time.Duration
}

// NewController creates a new controller with the specified options
func NewController(opts *Opts, stopChan <-chan struct{}) *Controller {
	client := NewClient(opts.K8SClient, opts.Customers, stopChan)
	if client == nil {
		log.Fatalln("Failed to create controller client")
		return nil
	}
	return &Controller{
		Client:   client,
		Opts:     opts,
		stopChan: stopChan,

		taintTracker: make(map[string][]string),
	}
}

// GetOpts is a helper to return the options for a customerName
func (c Controller) GetOpts(customerName string) (*NodeGroup, error) {
	if opts, ok := c.Opts.Customers[customerName]; ok {
		return opts, nil
	}
	return nil, errors.New(fmt.Sprintln("Failed to get node group for customer ", customerName))
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
func (c Controller) taintOldestN(nodes []*v1.Node, nodeGroup *NodeGroup, n int) []int {
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
		if !nodeGroup.DryMode {
			log.WithField("drymode", "off").Infoln("Tainting node", bundle.node.Name)
			updatedNode, err := k8s.AddToBeRemovedTaint(bundle.node, c.Client)
			if err != nil {
				bundle.node = updatedNode
				taintedIndices = append(taintedIndices, bundle.index)
			}
		} else {
			c.taintTracker[nodeGroup.Name] = append(c.taintTracker[nodeGroup.Name], bundle.node.Name)
			taintedIndices = append(taintedIndices, bundle.index)
			log.WithField("drymode", "on").Infoln("Tainting node", bundle.node.Name)
		}
	}

	return taintedIndices
}

func (c Controller) untaintNewestN(nodes []*v1.Node, nodeGroup *NodeGroup, n int) []int {
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
		if !nodeGroup.DryMode {
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
			for i, name := range c.taintTracker[nodeGroup.Name] {
				if bundle.node.Name == name {
					deleteIndex = i
					break
				}
			}
			if deleteIndex != -1 {
				// Delete from tracker
				c.taintTracker[nodeGroup.Name] = append(c.taintTracker[nodeGroup.Name][:deleteIndex], c.taintTracker[nodeGroup.Name][deleteIndex+1:]...)
				untaintedIndices = append(untaintedIndices, bundle.index)
				log.WithField("drymode", "on").Infoln("Untainting node", bundle.node.Name)
			}
		}
	}

	return untaintedIndices
}

func (c Controller) scaleNodeGroup(customer string, lister *NodeGroupLister) {
	opts, err := c.GetOpts(customer)
	if err != nil {
		log.Errorf("Failed to local customer: %v", err)
		return
	}

	// list all pods
	pods, err := lister.Pods.List()
	if err != nil {
		log.Errorf("Failed to list pods: %v", err)
		return
	}

	// List all nodes
	allNodes, err := lister.Nodes.List()
	if err != nil {
		log.Errorf("Failed to list nodes: %v", err)
		return
	}

	// Filter out tainted nodes
	untaintedNodes := make([]*v1.Node, 0, len(allNodes))
	for _, node := range allNodes {
		if opts.DryMode {
			var contains bool
			for _, name := range c.taintTracker[customer] {
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
	if math.Max(cpuPercent, memPercent) < float64(opts.TaintUpperCapacityThreshholdPercent) && len(untaintedNodes) > opts.MinNodes {
		log.Warningln("Upper threshhold reached. Scale down 1 node")
		metrics.NodeGroupTaintEvent.WithLabelValues(customer).Inc()
		nodesTainted := c.taintOldestN(untaintedNodes, opts, 1)

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
		if math.Max(cpuPercent, memPercent) < float64(opts.TaintLowerCapacityThreshholdPercent) && len(filtered)-2 > opts.MinNodes {
			log.Warningln("Lower threshhold reached. Scale down 1 node")
			metrics.NodeGroupTaintEvent.WithLabelValues(customer).Inc()
			c.taintOldestN(filtered, opts, 2)
		}
	}

	// Scale up by upper threshhold
	if math.Max(cpuPercent, memPercent) > float64(opts.UntaintLowerCapacityThreshholdPercent) && len(untaintedNodes)+1 < opts.MaxNodes {
		log.Warningln("Slack space reached. Trying to untaint a tainted node")
		metrics.NodeGroupTaintEvent.WithLabelValues(customer).Dec()
		// push in allNodes here to include tainted ones. Otherwise it will untaint nothing
		nodesUntainted := c.untaintNewestN(allNodes, opts, 1)

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
		if math.Max(cpuPercent, memPercent) > float64(opts.UntaintUpperCapacityThreshholdPercent) && len(filtered)+2 > opts.MinNodes {
			log.Warningln("Agressive Slack space reached. Trying to untaint tainted nodes")
			metrics.NodeGroupTaintEvent.WithLabelValues(customer).Inc()
			c.untaintNewestN(filtered, opts, 2)
		}
	}
}

// RunOnce performs the main autoscaler logic once
func (c Controller) RunOnce() {
	startTime := time.Now()

	// TODO(jgonzalez/dangot):
	// REAPER GOES HERE

	// Perform the ScaleUp/Taint logic
	for customer, lister := range c.Client.Listers {
		c.scaleNodeGroup(customer, lister)
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
