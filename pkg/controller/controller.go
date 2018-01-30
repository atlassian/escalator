package controller

import (
	"sync"
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

func (c Controller) scaleLogic(customer string, lister *NodeGroupLister, wait *sync.WaitGroup) {
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
	log.With("customer", customer).Infof("cpu: %v, memory: %v", cpuPercent, memPercent)
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
		go c.scaleLogic(customer, lister, &wait)
	}

	wait.Wait()

	endTime := time.Now()
	log.Infof("Scaling took a total of %v", endTime.Sub(startTime))
}

// RunForever starts the autoscaler process and runs once every ScanInterval. blocks thread
func (c Controller) RunForever(runImmediately bool, stop <-chan struct{}) {
	if runImmediately {
		c.RunOnce()
	}

	// Start the main loop
	ticker := time.NewTicker(c.Opts.ScanInterval)
	for {
		select {
		case <-ticker.C:
			log.Infoln("---[AUTOSCALER LOOP]---")
			c.RunOnce()
		case <-stop:
			return
		}
	}
}
