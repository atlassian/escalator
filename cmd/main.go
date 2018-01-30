package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/atlassian/escalator/pkg/controller"
	"github.com/atlassian/escalator/pkg/k8s"
	"gopkg.in/alecthomas/kingpin.v2"

	log "github.com/sirupsen/logrus"
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

		Customers: []*controller.NodeGroup{
			&controller.NodeGroup{
				Name:       "default",
				LabelValue: "shared",
				LabelKey:   "customer",
			},
			&controller.NodeGroup{
				Name:       "buildeng",
				LabelValue: "buildeng",
				LabelKey:   "customer",
			},
		},
	}

	signalChan := make(chan os.Signal, 1)
	stopChan := make(chan struct{})

	// Handle termination signals and shutdown gracefully
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-signalChan
		log.Infof("Signal received: %v", sig)
		log.Infoln("Stopping autoscaler gracefully")
		stopChan <- struct{}{}
	}()

	c := controller.NewController(opts)
	c.RunForever(true, stopChan)
}
