package main

import (
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

var (
	counter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "test",
			Help: "test",
		},
	)
)

func main() {
	log.Printf("%#v", kubernetes.Clientset{})
}
