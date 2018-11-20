package k8s

import (
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// NewOutOfClusterClient returns a new kubernetes clientset using a kubeconfig file
// For running outside the cluster
func NewOutOfClusterClient(kubeconfig string) (*kubernetes.Clientset, error) {
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, errors.Errorf("Failed to create out of cluster config: %v", err)
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Errorf("Failed to create out of cluster client: %v", err)
	}
	return clientset, nil
}

// NewInClusterClient returns a new kubernetes clientset from inside the cluster
func NewInClusterClient() (*kubernetes.Clientset, error) {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, errors.Errorf("Failed to create in of cluster config: %v", err)
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Errorf("Failed to create in of cluster client: %v", err)
	}
	return clientset, nil
}
