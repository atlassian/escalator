package k8s

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	v1lister "k8s.io/client-go/listers/core/v1"
)

// PodLister provides an interface for anything that can list a pod
type PodLister interface {
	List() ([]*v1.Pod, error)
}

// FilteredPodsLister lists pods from a podLister and filters out by namespace
type FilteredPodsLister struct {
	namespaces []string
	podLister  v1lister.PodLister
}

// NewFilteredPodsLister creates a new lister and informerSynced for a FilteredPodsLister
func NewFilteredPodsLister(podLister v1lister.PodLister, namespaces []string) PodLister {
	return &FilteredPodsLister{
		namespaces,
		podLister,
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
	// only include pods that are in one of the namespaces of a customer
	for _, pod := range allPods {
		for _, namespace := range lister.namespaces {
			if pod.Namespace == namespace {
				filteredPods = append(filteredPods, pod)
				break
			}
		}
	}

	return filteredPods, nil
}
