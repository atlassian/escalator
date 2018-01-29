package controller

import (
	"github.com/atlassian/escalator/pkg/k8s"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	v1lister "k8s.io/client-go/listers/core/v1"
)

// Client provides a wrapper around a k8s client that includes
// watching pods and nodes from cache
type Client struct {
	kubernetes.Interface
	Listers map[string]*CustomerLister

	// Backing store for all listers used by the Client
	allPodLister  v1lister.PodLister
	allNodeLister v1lister.NodeLister
}

// Customer represents a model for filtering pods and nodes
type Customer struct {
	Name       string
	Namespaces []string
	NodeLabels []string
}

// NewClient creates a new Client wrapper over the k8sclient with some pod and node listers
// It will wait for the cache to sync
func NewClient(k8sClient kubernetes.Interface, customers []*Customer) *Client {
	allPodLister, podSync := k8s.NewCachePodWatcher(k8sClient)
	allNodeLister, nodeSync := k8s.NewCacheNodeWatcher(k8sClient)

	synced := k8s.WaitForSync(3, podSync, nodeSync)
	if !synced {
		log.Fatalf("Attempted to wait for caches to be synced for %d however it is not done.  Giving up.", 3)
	} else {
		log.Debugln("Caches have been synced. Proceeding with server.")
	}

	customerMap := make(map[string]*CustomerLister)
	for _, customer := range customers {
		customerMap[customer.Name] = NewCustomerLister(allPodLister, allNodeLister, customer)
	}

	client := Client{
		k8sClient,
		customerMap,
		allPodLister,
		allNodeLister,
	}

	return &client
}
