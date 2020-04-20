package k8s

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	v1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

// NewCachePodWatcher creates a new IndexerInformer for watching pods from cache
func NewCachePodWatcher(client kubernetes.Interface, stop <-chan struct{}) (v1lister.PodLister, cache.InformerSynced) {
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
		cache.Indexers{},
	)
	podLister := v1lister.NewPodLister(podIndexer)
	go podController.Run(stop)
	return podLister, podController.HasSynced
}

// NewCacheNodeWatcher creates a new IndexerInformer for watching nodes from cache
func NewCacheNodeWatcher(client kubernetes.Interface, stop <-chan struct{}) (v1lister.NodeLister, cache.InformerSynced) {
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
		cache.Indexers{},
	)
	nodeLister := v1lister.NewNodeLister(nodeIndexer)
	go nodeController.Run(stop)
	return nodeLister, nodeController.HasSynced
}

// WaitForSync wait for the cache sync for all the registered listers
// it will try <tries> times and return the result
func WaitForSync(tries int, stopChan <-chan struct{}, informers ...cache.InformerSynced) bool {
	synced := false
	for i := 0; i < tries && !synced; i++ {
		log.Debugf("Trying to sync cache: tries = %v, max = %v", i, tries)
		synced = cache.WaitForCacheSync(stopChan, informers...)
	}
	return synced
}
