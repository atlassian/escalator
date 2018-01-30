package controller

import (
	"time"

	"github.com/atlassian/escalator/pkg/k8s"
	"github.com/prometheus/common/log"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
)

// Controller contains the core logic of the Autoscaler
type Controller struct {
	*Client
	*Opts
}

// Opts provide the Controller with config for runtime
type Opts struct {
	Addr         string
	ScanInterval time.Duration
	K8SClient    *kubernetes.Clientset
	Customers    []*NodeGroup
}

// NewController creates a new controller with the specified options
func NewController(opts *Opts) *Controller {
	client := NewClient(opts.K8SClient, opts.Customers)
	if client == nil {
		log.Fatalln("Failed to create controller client")
		return nil
	}
	return &Controller{
		Client: client,
		Opts:   opts,
	}
}

//
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

// RunOnce performs the main autoscaler logic once
func (c Controller) RunOnce() error {
	log.Infoln("pods\t\tnodes")

	for customer, lister := range c.Client.Listers {
		pods, err := lister.Pods.List()
		nodes, _ := lister.Nodes.List()
		if err != nil {
			log.Error(err)
		}
		memRequest, cpuRequest, err := k8s.CalculatePodsRequestsTotal(pods)
		memCapacity, cpuCapacity, err := k8s.CalculateNodesCapacityTotal(nodes)
		// log.With("customer", customer).Debugf("cpu:%s , memory:%s", cpuRequest.String(), memRequest.String())
		// log.With("customer", customer).Debugf("cpuCapacity:%s , memoryCapacity:%s", cpuCapacity.String(), memCapacity.String())

		cpuPercent, memPercent, err := calcPercentUsage(cpuRequest, memRequest, cpuCapacity, memCapacity)
		log.With("customer", customer).Infof("cpu: %v, memory: %v", cpuPercent, memPercent)

	}
	return nil
}

// RunForever starts the autoscaler process and runs once every ScanInterval. blocks thread
func (c Controller) RunForever(runImmediately bool, stop <-chan struct{}) {
	if runImmediately {
		if err := c.RunOnce(); err != nil {
			log.Errorf("Error occured during first execution: %v", err)
		}
	}

	// Start the main loop
	ticker := time.NewTicker(c.Opts.ScanInterval)
	for {
		select {
		case <-ticker.C:
			log.Debugln("---[AUTOSCALER LOOP]---")
			if err := c.RunOnce(); err != nil {
				log.Errorf("Error occured during execution: %v", err)
			}
		case <-stop:
			return
		}
	}
}
