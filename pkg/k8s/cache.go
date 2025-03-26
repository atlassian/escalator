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
func NewCachePodWatcher(client kubernetes.Interface, stop <-chan struct{}) (v1lister.PodLister, cache.InformerSynced, error) {
	selector := fields.ParseSelectorOrDie(fmt.Sprint("status.phase!=", v1.PodSucceeded, ",status.phase!=", v1.PodFailed))
	podsListWatch := cache.NewListWatchFromClient(
		client.CoreV1().RESTClient(),
		"pods",
		v1.NamespaceAll,
		selector,
	)
	podStore, podController := cache.NewInformerWithOptions(cache.InformerOptions{
		ListerWatcher: podsListWatch,
		ObjectType:    &v1.Pod{},
		Handler:       cache.ResourceEventHandlerFuncs{},
		ResyncPeriod:  1 * time.Hour,
		Indexers:      cache.Indexers{},
	})
	podIndexer, ok := podStore.(cache.Indexer)
	if !ok {
		return nil, nil, fmt.Errorf( "expected Indexer, but got a Store that does not implement Indexer")
	}
	podLister := v1lister.NewPodLister(podIndexer)
	go podController.Run(stop)
	return podLister, podController.HasSynced, nil
}

// NewCacheNodeWatcher creates a new IndexerInformer for watching nodes from cache
func NewCacheNodeWatcher(client kubernetes.Interface, stop <-chan struct{}) (v1lister.NodeLister, cache.InformerSynced, error) {
	selector := fields.Everything()
	nodesListWatch := cache.NewListWatchFromClient(
		client.CoreV1().RESTClient(),
		"nodes",
		v1.NamespaceAll,
		selector,
	)
	nodeStore, nodeController := cache.NewInformerWithOptions(cache.InformerOptions{
		ListerWatcher: nodesListWatch,
		ObjectType:    &v1.Node{},
		Handler:       cache.ResourceEventHandlerFuncs{},
		ResyncPeriod:  1 * time.Hour,
		Indexers:      cache.Indexers{},
	})
	nodeIndexer, ok := nodeStore.(cache.Indexer)
	if !ok {
		return nil, nil, fmt.Errorf( "expected Indexer, but got a Store that does not implement Indexer")
	}
	nodeLister := v1lister.NewNodeLister(nodeIndexer)
	go nodeController.Run(stop)
	return nodeLister, nodeController.HasSynced, nil
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
