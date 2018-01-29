package k8s

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	v1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

func NewCachePodWatcher(client kubernetes.Interface) (v1lister.PodLister, cache.InformerSynced) {
	selector := fields.ParseSelectorOrDie(fmt.Sprint("status.phase!=", v1.PodSucceeded, ",status.phase!=", v1.PodFailed))
	podsListWatch := cache.NewListWatchFromClient(
		client.CoreV1().RESTClient(),
		"pods",
		v1.NamespaceAll,
		selector,
	)
	podIndexer, podController := cache.NewIndexerInformer(
		podsListWatch,
		&v1.Pod{},
		1*time.Hour,
		cache.ResourceEventHandlerFuncs{},
		cache.Indexers{
			cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
		},
	)
	podLister := v1lister.NewPodLister(podIndexer)
	go podController.Run(wait.NeverStop)
	return podLister, podController.HasSynced
}

func NewCacheNodeWatcher(client kubernetes.Interface) (v1lister.NodeLister, cache.InformerSynced) {
	selector := fields.Everything()
	nodesListWatch := cache.NewListWatchFromClient(
		client.CoreV1().RESTClient(),
		"nodes",
		v1.NamespaceAll,
		selector,
	)
	nodeIndexer, nodeController := cache.NewIndexerInformer(
		nodesListWatch,
		&v1.Node{},
		1*time.Hour,
		cache.ResourceEventHandlerFuncs{},
		cache.Indexers{
			cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
		},
	)
	nodeLister := v1lister.NewNodeLister(nodeIndexer)
	go nodeController.Run(wait.NeverStop)
	return nodeLister, nodeController.HasSynced
}

// WaitForSync wait for the cache sync for all the registered listers
// it will try <tries> times and return the result
func WaitForSync(tries int, informers ...cache.InformerSynced) bool {
	synced := false
	for i := 0; i < tries && !synced; i++ {
		synced = cache.WaitForCacheSync(nil, informers...)
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
