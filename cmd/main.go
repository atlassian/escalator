package main

import (
	"time"

	log "github.com/sirupsen/logrus"

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

	k8sClient := k8s.NewOutOfClusterClient(*kubeconfig)
	client := k8s.NewClient(k8sClient)

	for {
		pods, err := client.Listers.AllPods.List()
		spods, _ := client.Listers.ScheduledPods.List()
		upods, _ := client.Listers.UnschedulablePods.List()
		nodes, _ := client.Listers.AllNodes.List()
		if err != nil {
			log.Error(err)
		}
		log.Infoln("all\tscheduled\tunscheduable\tnodes")
		log.Infof("%v\t%v\t%v\t%v", len(pods), len(spods), len(upods), len(nodes))
		time.Sleep(1 * time.Second)
	}

	// http.Handle("/metrics", promhttp.Handler())
	// go http.ListenAndServe(*addr, nil)

	// opts := &controller.Opts{
	// 	*addr,
	// 	*scanInterval,
	// 	*kubeconfig,
	// }

	// c := controller.NewController(opts)
	// c.Run()
}
