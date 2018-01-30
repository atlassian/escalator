package controller

import (
	"time"

	"github.com/prometheus/common/log"
	"k8s.io/client-go/kubernetes"
	"github.com/atlassian/escalator/pkg/k8s"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Controller contains the core logic of the Autoscaler
type Controller struct {
	Client *Client
	ScanInterval time.Duration
}

// Opts provide the Controller with config for runtime
type Opts struct {
	Addr         string
	ScanInterval time.Duration
	K8SClient    *kubernetes.Clientset
	Customers	[]*NodeGroup
}

// NewController creates a new controller with the specified options
func NewController(opts *Opts) *Controller {
	testClient := NewClient(opts.K8SClient, []*NodeGroup{
		&NodeGroup{
			Name:       "default",
			LabelValue: "shared",
			LabelKey: "customer",
		},
		&NodeGroup{
			Name:       "buildeng",
			LabelValue: "buildeng",
			LabelKey: "customer",
		},
	})
	return &Controller{
		Client: testClient,
		ScanInterval: opts.ScanInterval,
	}
}

//
func calcPercentUsage(cpuR, memR, cpuA, memA resource.Quantity) (float64, float64, error) {

	cpuPercent := float64(cpuR.MilliValue()) / float64(cpuA.MilliValue())  * 100
	memPercent := float64(memR.MilliValue()) / float64(memA.MilliValue())  * 100
	return cpuPercent, memPercent, nil
}

func doesItFit(cpuR, memR, cpuA, memA resource.Quantity) (bool, error){
	if cpuR.Cmp(cpuA) == -1{
		log.Infof("There is enough CPU")
	}

	if memR.Cmp(memA) == -1{
		log.Infof("There is enough Memory")
	}

	return true, nil
}


// Run starts the autoscaler process and blocks
func (c Controller) Run() error {
	// Testing stuff
	// TODO: use a proper ticker
	log.Infoln("pods\t\tnodes")
	for {
		log.Info("[Loop]----")
		for customer, lister := range c.Client.Listers {
			pods, err := lister.Pods.List()
			nodes, _ := lister.Nodes.List()
			if err != nil {
				log.Error(err)
			}
			memRequest, cpuRequest, err := k8s.CalculateRequests(pods)
			memCapacity, cpuCapacity, err := k8s.CalculateNodesCapacity(nodes)
			// log.With("customer", customer).Debugf("cpu:%s , memory:%s", cpuRequest.String(), memRequest.String())
			// log.With("customer", customer).Debugf("cpuCapacity:%s , memoryCapacity:%s", cpuCapacity.String(), memCapacity.String())

			cpuPercent, memPercent, err := calcPercentUsage(cpuRequest, memRequest, cpuCapacity, memCapacity)
			log.With("customer", customer).Infof("cpu: %v, memory: %v", cpuPercent, memPercent)

		}
		time.Sleep(c.ScanInterval)
	}
	return nil
}
