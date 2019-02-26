package k8s

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	v1lister "k8s.io/client-go/listers/core/v1"
)

// PodFilterFunc provides a definition for a predicate based on matching a pod
// return true for keep node
type PodFilterFunc func(*v1.Pod) bool

// PodLister provides an interface for anything that can list a pod
type PodLister interface {
	List() ([]*v1.Pod, error)
}

// FilteredPodsLister lists pods from a podLister and filters out by namespace
type FilteredPodsLister struct {
	podLister  v1lister.PodLister
	filterFunc PodFilterFunc
}

// NewFilteredPodsLister creates a new lister and informerSynced for a FilteredPodsLister
func NewFilteredPodsLister(podLister v1lister.PodLister, filterFunc PodFilterFunc) PodLister {
	return &FilteredPodsLister{
		podLister,
		filterFunc,
	}
}

// List lists all pods from the cache filtering by namespace
func (lister *FilteredPodsLister) List() ([]*v1.Pod, error) {
	var filteredPods []*v1.Pod
	allPods, err := lister.podLister.List(labels.Everything())
	if err != nil {
		return filteredPods, err
	}

	filteredPods = make([]*v1.Pod, 0, len(allPods))
	// only include pods that match the filtering function
	for _, pod := range allPods {
		if lister.filterFunc(pod) {
			filteredPods = append(filteredPods, pod)
		}
	}

	return filteredPods, nil
}
