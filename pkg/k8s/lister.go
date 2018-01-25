package k8s

import (
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	v1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

// PodLister provides an interface for anything that can list a pod
type PodLister interface {
	List() ([]*v1.Pod, error)
}

// AllPodsLister lists all pods
type AllPodsLister struct {
	podLister v1lister.PodLister
	stopChan  chan struct{}
}

// NewAllPodsLister creates a lister that lists all pods in a given namespace
func NewAllPodsLister(client kubernetes.Interface, namespace string, stopChan <-chan struct{}) PodLister {
	selector := fields.Everything()
	podsListWatch := cache.NewListWatchFromClient(
		client.CoreV1().RESTClient(),
		"pods",
		namespace,
		selector,
	)
	store := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
	})
	podLister := v1lister.NewPodLister(store)
	podReflector := cache.NewReflector(
		podsListWatch,
		&v1.Pod{},
		store,
		time.Hour,
	)
	go podReflector.Run(stopChan)
	return &AllPodsLister{
		podLister: podLister,
	}
}

// List all pods regardless
func (lister *AllPodsLister) List() ([]*v1.Pod, error) {
	return lister.podLister.List(labels.Everything())
}
