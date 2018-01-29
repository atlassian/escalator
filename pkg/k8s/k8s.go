package k8s

import (
	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
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
	// Pod listers
	AllPods           PodLister
	ScheduledPods     PodLister
	UnschedulablePods PodLister

	// Node listers
	AllNodes NodeLister

	// informers for cache syncing
	informers []cache.InformerSynced
}

// WaitForSync wait for the cache sync for all the registered listers
// it will try <tries> times and return the result
func (lg *ListerGroup) WaitForSync(tries int) bool {
	synced := false
	for i := 0; i < 10 && !synced; i++ {
		synced = cache.WaitForCacheSync(nil, lg.informers...)
	}
	return synced
}

// NewOutOfClusterClient returns a new kubernetes clientset using a kubeconfig file
// For running outside the cluster
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

// NewInClusterClient returns a new kubernetes clientset from inside the cluster
func NewInClusterClient() *kubernetes.Clientset {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Failed to create in of cluster config: %v", err)
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create in of cluster client: %v", err)
	}
	return clientset
}

// NewClient creates a new Client wrapper over the k8sclient with some pod and node listers
// It will wait for the cache to sync
func NewClient(k8sClient kubernetes.Interface) *Client {
	var allInformers []cache.InformerSynced

	// create the pods lister for all pods
	allPodsLister, allPodsInformer := NewAllPodsLister(k8sClient, v1.NamespaceAll)
	allInformers = append(allInformers, allPodsInformer)

	// create the pods lister for scheduled pods
	scheduledPodsLister, scheduledPodsInformer := NewScheduledPodsLister(k8sClient, v1.NamespaceAll)
	allInformers = append(allInformers, scheduledPodsInformer)

	// create the pods lister for unschedulable pods
	unschedulablePodsLister, unschedulablePodsInformer := NewUnschedulablePodsLister(k8sClient, v1.NamespaceAll)
	allInformers = append(allInformers, unschedulablePodsInformer)

	// create the node lister for all nodes
	allNodesLister, allNodesInformer := NewAllNodesLister(k8sClient)
	allInformers = append(allInformers, allNodesInformer)

	listers := &ListerGroup{
		AllPods:           allPodsLister,
		ScheduledPods:     scheduledPodsLister,
		UnschedulablePods: unschedulablePodsLister,

		AllNodes: allNodesLister,

		informers: allInformers,
	}

	synced := listers.WaitForSync(3)
	if !synced {
		log.Fatalf("Attempted to wait for caches to be synced for %d however it is not done.  Giving up.", 3)
	} else {
		log.Debugln("Caches have been synced. Proceeding with server.")
	}

	client := Client{
		k8sClient,
		listers,
	}

	return &client
}
