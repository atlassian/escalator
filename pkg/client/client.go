package client

import (
	"github.com/atlassian/escalator/pkg/k8s"
	"github.com/atlassian/escalator/pkg/lister"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	v1lister "k8s.io/client-go/listers/core/v1"
)

// Client provides a wrapper around a k8s client that includes
// watching pods and nodes from cache
type Client struct {
	kubernetes.Interface
	Listers map[string]*lister.ListerGroup

	allPodLister  v1lister.PodLister
	allNodeLister v1lister.NodeLister
}

type Customer struct {
	Name       string
	Namespaces []string
	NodeLabels []string
}

func (c *Client) ListGroupFromCustomer(customer *Customer) lister.Group {
	return lister.NewGroup(c.allPodLister, c.allNodeLister, customer.Namespaces, customer.NodeLabels)
}

// NewClient creates a new Client wrapper over the k8sclient with some pod and node listers
// It will wait for the cache to sync
func NewClient(k8sClient kubernetes.Interface) *Client {
	allPodLister, podSync := k8s.NewCachePodWatcher(k8sClient)
	allNodeLister, nodeSync := k8s.NewCacheNodeWatcher(k8sClient)

	synced := k8s.WaitForSync(3, podSync, nodeSync)
	if !synced {
		log.Fatalf("Attempted to wait for caches to be synced for %d however it is not done.  Giving up.", 3)
	} else {
		log.Debugln("Caches have been synced. Proceeding with server.")
	}

	client := Client{
		k8sClient,
		map[string]*lister.ListerGroup{},
		allPodLister,
		allNodeLister,
	}

	return &client
}
