package k8s

import (
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// DrainPods attempts to delete or evict pods so that the node can be terminated
// Will prioritise using Evict if kubernetes supports it.
func DrainPods(pods []*v1.Pod, client kubernetes.Interface) error {
	// Determine whether we are able to delete or evict pods
	apiVersion, err := SupportEviction(client)
	useEviction := len(apiVersion) > 0
	if err != nil {
		return err
	}

	// If we are able to evict
	if useEviction {
		return EvictPods(pods, apiVersion, client)
	} else {
		// Otherwise delete pods
		return DeletePods(pods, client)
	}
}
