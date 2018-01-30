package controller

import (
	"time"

	"github.com/atlassian/escalator/pkg/k8s"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	v1lister "k8s.io/client-go/listers/core/v1"
)

// Client provides a wrapper around a k8s client that includes
// anything needed by the controller for listing customer pods and nodes based on a filter
type Client struct {
	kubernetes.Interface
	Listers map[string]*NodeGroupLister

	// Backing store for all listers used by the Client
	allPodLister  v1lister.PodLister
	allNodeLister v1lister.NodeLister
}

// NewClient creates a new client wrapper over the k8sclient with some pod and node listers
// It will wait for the cache to sync before returning
func NewClient(k8sClient kubernetes.Interface, customers []*NodeGroup) *Client {
	// Backing store lister for all pods and nodes
	allPodLister, podSync := k8s.NewCachePodWatcher(k8sClient)
	allNodeLister, nodeSync := k8s.NewCacheNodeWatcher(k8sClient)

	startTime := time.Now()
	log.Infoln("Waiting for cache to sync...")
	synced := k8s.WaitForSync(3, podSync, nodeSync)
	if !synced {
		log.Fatalf("Attempted to wait for caches to be synced for %d however it is not done.  Giving up.", 3)
	}
	endTime := time.Now()
	log.Infof("Cache took %v to sync", endTime.Sub(startTime))

	// load in all our node group listers from our customers
	customerMap := make(map[string]*NodeGroupLister)

	for _, customer := range customers {
		if customer.Name == DefaultCustomer {
			customerMap[customer.Name] = NewDefaultNodeGroupLister(allPodLister, allNodeLister, customer)
		} else {
			customerMap[customer.Name] = NewNodeGroupLister(allPodLister, allNodeLister, customer)
		}
	}
	client := Client{
		k8sClient,
		customerMap,
		allPodLister,
		allNodeLister,
	}

	return &client
}
