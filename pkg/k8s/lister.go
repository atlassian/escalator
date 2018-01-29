package k8s

import (
	"fmt"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	v1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	podv1 "k8s.io/kubernetes/pkg/api/v1/pod"
)

// PodLister provides an interface for anything that can list a pod
type PodLister interface {
	List() ([]*v1.Pod, error)
}

// AllPodsLister lists all pods regardless of state
type AllPodsLister struct {
	podLister v1lister.PodLister
}

// NewAllPodsLister creates a new lister and informerSynced for all pods
func NewAllPodsLister(client kubernetes.Interface, namespace string) (PodLister, cache.InformerSynced) {
	selector := fields.Everything()
	podsListWatch := cache.NewListWatchFromClient(
		client.CoreV1().RESTClient(),
		"pods",
		namespace,
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
	return &AllPodsLister{
		podLister,
	}, podController.HasSynced
}

// List lists all pods from the cache
func (lister *AllPodsLister) List() ([]*v1.Pod, error) {
	return lister.podLister.List(labels.Everything())
}

// ScheduledPodsLister lists all pods that are currently scheduled
type ScheduledPodsLister struct {
	podLister v1lister.PodLister
}

// NewScheduledPodsLister creates a new lister and informerSynced for scheduled pods
func NewScheduledPodsLister(client kubernetes.Interface, namespace string) (PodLister, cache.InformerSynced) {
	selector := fields.ParseSelectorOrDie(fmt.Sprint("spec.nodeName!=\"\",status.phase!=", v1.PodSucceeded, ",status.phase!=", v1.PodFailed))
	podsListWatch := cache.NewListWatchFromClient(
		client.CoreV1().RESTClient(),
		"pods",
		namespace,
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
	return &ScheduledPodsLister{
		podLister,
	}, podController.HasSynced
}

// List lists all pods from the cache
func (lister *ScheduledPodsLister) List() ([]*v1.Pod, error) {
	return lister.podLister.List(labels.Everything())
}

// UnschedulablePodsLister lists all pods that are currently unschedulable
type UnschedulablePodsLister struct {
	podLister v1lister.PodLister
}

// NewUnschedulablePodsLister creates a new lister and informerSynced for unschedulable pods
func NewUnschedulablePodsLister(client kubernetes.Interface, namespace string) (PodLister, cache.InformerSynced) {
	selector := fields.ParseSelectorOrDie(fmt.Sprint("spec.nodeName==\"\",status.phase!=", v1.PodSucceeded, ",status.phase!=", v1.PodFailed))
	podsListWatch := cache.NewListWatchFromClient(
		client.CoreV1().RESTClient(),
		"pods",
		namespace,
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
	return &UnschedulablePodsLister{
		podLister,
	}, podController.HasSynced
}

// List lists all pods from the cache
func (lister *UnschedulablePodsLister) List() ([]*v1.Pod, error) {
	var unschedulablePods []*v1.Pod
	allPods, err := lister.podLister.List(labels.Everything())
	if err != nil {
		return unschedulablePods, err
	}
	for _, pod := range allPods {
		_, cond := podv1.GetPodCondition(&pod.Status, v1.PodScheduled)
		if cond != nil && cond.Status == v1.ConditionFalse && cond.Reason == v1.PodReasonUnschedulable {
			unschedulablePods = append(unschedulablePods, pod)
		}
	}
	return unschedulablePods, nil
}

// NodeLister provides an interface for anything that can list a pod
type NodeLister interface {
	List() ([]*v1.Node, error)
}

// AllNodesLister lists all nodes regardless of state
type AllNodesLister struct {
	nodeLister v1lister.NodeLister
}

// NewAllNodesLister creates a new lister and informerSynced for all nodes
func NewAllNodesLister(client kubernetes.Interface) (NodeLister, cache.InformerSynced) {
	selector := fields.Everything()
	podsListWatch := cache.NewListWatchFromClient(
		client.CoreV1().RESTClient(),
		"nodes",
		v1.NamespaceAll,
		selector,
	)
	nodeIndexer, podController := cache.NewIndexerInformer(
		podsListWatch,
		&v1.Node{},
		1*time.Hour,
		cache.ResourceEventHandlerFuncs{},
		cache.Indexers{
			cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
		},
	)
	nodeLister := v1lister.NewNodeLister(nodeIndexer)
	go podController.Run(wait.NeverStop)
	return &AllNodesLister{
		nodeLister,
	}, podController.HasSynced
}

// List lists all pods from the cache
func (lister *AllNodesLister) List() ([]*v1.Node, error) {
	return lister.nodeLister.List(labels.Everything())
}
