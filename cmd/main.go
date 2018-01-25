package main

import (
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/atlassian/escalator/pkg/controller"
	"github.com/atlassian/escalator/pkg/k8s"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	// loglevel = kingpin.Flag("loglevel", "Verbose mode.").Short('v').Default("INFO").String()
	addr         = kingpin.Flag("address", "Address to listen to for /metrics").Default(":8080").String()
	scanInterval = kingpin.Flag("scaninterval", "How often cluster is reevaluated for scale up or down").Default("60s").Duration()
	kubeconfig   = kingpin.Flag("kubeconfig", "Kubeconfig file location").String()
)

func main() {

	kingpin.Parse()

	k8sClient := k8s.NewOutOfClusterClient(*kubeconfig)
	client := k8s.NewClient(k8sClient)

	for {
		pods, err := client.Listers.AllPods.List()
		nodes, err := client.Listers.AllNodes.List()
		if err != nil {
			log.Error(err)
		}
		log.Infof("pod counts: %v: node counts: %v", len(pods), len(nodes))
		time.Sleep(1 * time.Second)
	}

	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(*addr, nil)

	opts := &controller.Opts{
		*addr,
		*scanInterval,
		*kubeconfig,
	}

	c := controller.NewController(opts)
	c.Run()
}
