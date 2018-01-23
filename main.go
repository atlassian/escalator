package main

import (
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

func main() {
	log.Printf("%#v", kubernetes.Clientset{})
}
