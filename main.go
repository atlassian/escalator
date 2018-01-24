package main

import (
	log "github.com/sirupsen/logrus"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/client-go/kubernetes"
)

func main() {
	b := kingpin.Arg("sd", "S").Bool()
	log.Printf("%#v", kubernetes.Clientset{})
	log.Println("Hello, world!", b)
}
