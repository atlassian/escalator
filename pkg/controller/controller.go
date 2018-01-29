package controller

import (
	"time"

	"github.com/prometheus/common/log"
	"k8s.io/client-go/kubernetes"
)

// Controller contains the core logic of the Autoscaler
type Controller struct {
	Client *Client
}

// Opts provide the Controller with config for runtime
type Opts struct {
	Addr         string
	ScanInterval time.Duration
	K8SClient    *kubernetes.Clientset
}

// NewController creates a new controller with the specified options
func NewController(opts *Opts) *Controller {
	testClient := NewClient(opts.K8SClient, []*Customer{
		&Customer{
			Name:       "kitt",
			Namespaces: []string{"kube-system", "monitoring"},
			NodeLabels: []string{"kitt"},
		},
		&Customer{
			Name:       "shared",
			Namespaces: []string{"default", "kube-public"},
			NodeLabels: []string{"shared"},
		},
	})
	return &Controller{
		Client: testClient,
	}
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
			log.With("customer", customer).Infof("%v\t%v", len(pods), len(nodes))
		}
		time.Sleep(1 * time.Second)
	}
	return nil
}
