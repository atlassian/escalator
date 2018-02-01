package controller

import (
	"math"
	"time"

	"github.com/atlassian/escalator/pkg/k8s"
	"github.com/atlassian/escalator/pkg/metrics"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"

	log "github.com/sirupsen/logrus"
)

var smoothingCoefficients = [...]float64{1, -8, 0, 8, -1}

// Controller contains the core logic of the Autoscaler
type Controller struct {
	*Client
	*Opts
	stopChan               <-chan struct{}
	customersMemoryHistory map[string][]float64
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
		Client:                 client,
		Opts:                   opts,
		stopChan:               stopChan,
		customersMemoryHistory: make(map[string][]float64),
	}
}

func calcPercentUsage(cpuR, memR, cpuA, memA resource.Quantity) (float64, float64, error) {
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

func (c Controller) scaleNodeGroup(customer string, lister *NodeGroupLister) {
	pods, err := lister.Pods.List()
	nodes, _ := lister.Nodes.List()
	if err != nil {
		log.Error(err)
		return
	}

	// Metrics
	metrics.NodeGroupNodes.WithLabelValues(customer).Set(float64(len(nodes)))
	metrics.NodeGroupPods.WithLabelValues(customer).Set(float64(len(pods)))

	memRequest, cpuRequest, err := k8s.CalculatePodsRequestsTotal(pods)
	memCapacity, cpuCapacity, err := k8s.CalculateNodesCapacityTotal(nodes)

	metrics.NodeGroupCPURequest.WithLabelValues(customer).Set(float64(cpuRequest.MilliValue()))
	bytesMemReq, _ := memRequest.AsInt64()
	metrics.NodeGroupMemRequest.WithLabelValues(customer).Set(float64(bytesMemReq))
	metrics.NodeGroupCPUCapacity.WithLabelValues(customer).Set(float64(cpuCapacity.MilliValue()))
	bytesMemCap, _ := memCapacity.AsInt64()
	metrics.NodeGroupMemCapacity.WithLabelValues(customer).Set(float64(bytesMemCap))

	cpuPercent, memPercent, err := calcPercentUsage(cpuRequest, memRequest, cpuCapacity, memCapacity)

	log.WithField("customer", customer).Infof("cpu: %v, memory: %v", cpuPercent, memPercent)
	metrics.NodeGroupsCPUPercent.WithLabelValues(customer).Set(cpuPercent)
	metrics.NodeGroupsMemPercent.WithLabelValues(customer).Set(memPercent)

	// TODO: Remove me. leaving in for metrics for now
	c.customersMemoryHistory[customer] = append(c.customersMemoryHistory[customer], memPercent)
	if len(c.customersMemoryHistory[customer]) >= len(smoothingCoefficients) {
		var deriv float64
		for i := range smoothingCoefficients {
			deriv += c.customersMemoryHistory[customer][i] * smoothingCoefficients[i]
		}
		deriv /= 12
		metrics.NodeGroupsMemPercentDeriv.WithLabelValues(customer).Set(deriv)
		c.customersMemoryHistory[customer] = c.customersMemoryHistory[customer][1:]
	}

	log.WithField("customer", customer).Infoln("upper", c.Opts.Customers[customer].UpperCapacityThreshholdPercent)
	if math.Max(cpuPercent, memPercent) < float64(c.Opts.Customers[customer].UpperCapacityThreshholdPercent) {
		log.Warningln("Scale down 1 node")
		metrics.NodeGroupTaintEvent.WithLabelValues(customer).Inc()
	}
	if math.Max(cpuPercent, memPercent) < float64(c.Opts.Customers[customer].LowerCapacityThreshholdPercent) {
		log.Warningln("Scale down 1 node")
		metrics.NodeGroupTaintEvent.WithLabelValues(customer).Inc()
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
