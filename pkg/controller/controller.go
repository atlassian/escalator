package controller

import (
	"time"

	"github.com/prometheus/common/log"
	"k8s.io/client-go/kubernetes"
)

type Controller struct {
	Client *Client
}

type Opts struct {
	Addr         string
	ScanInterval time.Duration
	K8SClient    *kubernetes.Clientset
}

func NewController(opts *Opts) *Controller {
	testClient := NewClient(opts.K8SClient, []*Customer{
		&Customer{
			Name:       "kitt",
			Namespaces: []string{"kube-system"},
			NodeLabels: []string{"shared", "kitt"},
		},
	})
	return &Controller{
		Client: testClient,
	}
}

func (c Controller) Run() error {
	log.Infoln("pods\t\tnodes")
	for {
		for customer, lister := range c.Client.Listers {
			log.Info("customer = ", customer)
			pods, err := lister.Pods.List()
			nodes, _ := lister.Nodes.List()
			if err != nil {
				log.Error(err)
			}
			log.Infof("%v\t%v", len(pods), len(nodes))
			time.Sleep(1 * time.Second)
		}
	}
	return nil
}
