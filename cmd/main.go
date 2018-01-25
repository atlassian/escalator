package main

import (
	"github.com/atlassian/escalator/pkg/k8s"
	// log "github.com/sirupsen/logrus"
	"net/http"

	"github.com/atlassian/escalator/pkg/controller"
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

	client := k8s.Client{}
	client.TestStuff()

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
