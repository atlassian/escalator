package k8s

import (
	"fmt"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	v1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
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
