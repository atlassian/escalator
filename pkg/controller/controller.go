package controller

import (
	"time"
	"github.com/prometheus/common/log"
)

type Controller struct {
	Autoscale string
	Awssession string
	Kubeclient string

}

type Opts struct {
	Addr string
	ScanInterval time.Duration
	Kubeconfig string
}

func NewController(opts *Opts) *Controller {
	return &Controller{
		Autoscale:  "",
		Awssession: "",
		Kubeclient: "",
	}
}

func (c Controller) Run() error {
	// time.sleep()
	// do something
	log.Info("Hello world")
	return nil
}
