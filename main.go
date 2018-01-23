package main

import (
	"github.com/aws/aws-sdk-go/aws/session"
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
	sess := session.Must(session.NewSession())
	log.Printf("%#v", sess.Config.Credentials)
	log.Printf("%#v", kubernetes.Clientset{})
}
