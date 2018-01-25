package k8s

import (
	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Client provides a wrapper around a k8s client that includes
// watching pods and nodes from cache
type Client struct {
	kubernetes.Interface
	Listers *ListerGroup
}

// ListerGroup is just a light wrapper around a few listers
type ListerGroup struct {
	AllPods     PodLister
	allPodsStop chan struct{}
}

// Shutdown will send the Stop() command to all listers in the group
func (lg *ListerGroup) Shutdown() {
	lg.allPodsStop <- struct{}{}
}

func NewOutOfClusterClient(kubeconfig string) *kubernetes.Clientset {
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("Failed to create out of cluster config: %v", err)
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create out of cluster client: %v", err)
	}
	return clientset
}

// NewClient creates a new Client wrapper over the k8sclient with some pod and node listers
func NewClient(k8sClient kubernetes.Interface) *Client {
	allPodsStopChan := make(chan struct{})
	listers := &ListerGroup{
		AllPods:     NewAllPodsLister(k8sClient, v1.NamespaceAll, allPodsStopChan),
		allPodsStop: allPodsStopChan,
	}

	client := Client{
		k8sClient,
		listers,
	}

	return &client
}
