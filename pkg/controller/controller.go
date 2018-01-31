package controller

import (
	"math"
	"sync"
	"time"

	"github.com/atlassian/escalator/pkg/k8s"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"

	log "github.com/sirupsen/logrus"
)

// Controller contains the core logic of the Autoscaler
type Controller struct {
	*Client
	*Opts
	stopChan <-chan struct{}
}

// Opts provide the Controller with config for runtime
type Opts struct {
	Addr         string
	ScanInterval time.Duration
	K8SClient    kubernetes.Interface
	Customers    []*NodeGroup
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

func (c Controller) scaleNodeGroup(customer string, lister *NodeGroupLister, wait *sync.WaitGroup) {
	defer wait.Done()

	pods, err := lister.Pods.List()
	nodes, _ := lister.Nodes.List()
	if err != nil {
		log.Error(err)
		return
	}

	memRequest, cpuRequest, err := k8s.CalculatePodsRequestsTotal(pods)
	memCapacity, cpuCapacity, err := k8s.CalculateNodesCapacityTotal(nodes)

	cpuPercent, memPercent, err := calcPercentUsage(cpuRequest, memRequest, cpuCapacity, memCapacity)
	log.WithField("customer", customer).Infof("cpu: %v, memory: %v", cpuPercent, memPercent)

	if math.Max(cpuPercent, memPercent) < 50.0 {
		log.Warningln("Scale down??")
	}
}

// RunOnce performs the main autoscaler logic once
func (c Controller) RunOnce() {
	startTime := time.Now()

	// TODO(jgonzalez/dangot): REAPER GOES HERE

	// Perform the ScaleUp/Taint logic
	// can perform scale logic for each customer in paralell
	// not a big deal with few customers, but for scalability if there are many
	var wait sync.WaitGroup
	wait.Add(len(c.Client.Listers))

	for customer, lister := range c.Client.Listers {
		go c.scaleNodeGroup(customer, lister, &wait)
	}

	wait.Wait()

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
