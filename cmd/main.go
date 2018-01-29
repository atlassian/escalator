package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/atlassian/escalator/pkg/controller"
	"github.com/atlassian/escalator/pkg/k8s"
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

	// for now we'll just use the out of cluster one for testing
	k8sClient := k8s.NewOutOfClusterClient(*kubeconfig)

	// endpoints
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(*addr, nil)

	opts := &controller.Opts{
		Addr:         *addr,
		ScanInterval: *scanInterval,
		K8SClient:    k8sClient,
	}

	c := controller.NewController(opts)
	c.Run()
}
