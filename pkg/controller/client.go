package controller

import (
	"time"

	"github.com/atlassian/escalator/pkg/k8s"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	v1lister "k8s.io/client-go/listers/core/v1"
)

// Client provides a wrapper around a k8s client that includes
// anything needed by the controller for listing nodegroup pods and nodes based on a filter
type Client struct {
	kubernetes.Interface
	Listers map[string]*NodeGroupLister

	// Backing store for all listers used by the Client
	allPodLister  v1lister.PodLister
	allNodeLister v1lister.NodeLister
}

// NewClient creates a new client wrapper over the k8sclient with some pod and node listers
// It will wait for the cache to sync before returning
func NewClient(k8sClient kubernetes.Interface, nodegroups []NodeGroupOptions, stopCache <-chan struct{}) (*Client, error) {
	// Backing store lister for all pods and nodes
	podStopChan := make(chan struct{})
	nodeStopChan := make(chan struct{})

	allPodLister, podSync := k8s.NewCachePodWatcher(k8sClient, podStopChan)
	allNodeLister, nodeSync := k8s.NewCacheNodeWatcher(k8sClient, nodeStopChan)

	// Spawn a routine to watch for the global stop signal
	// once it's received, send the stop signal to the cache informers
	go func() {
		<-stopCache
		log.Info("Stop signal received. Stopping cache watchers")
		close(podStopChan)
		close(nodeStopChan)
	}()

	log.Info("Waiting for cache to sync...")
	startTime := time.Now()

	synced := k8s.WaitForSync(3, stopCache, podSync, nodeSync)
	if !synced {
		return nil, errors.New("attempted to wait for caches to be synced %d times. Exiting")
	}

	endTime := time.Now()
	log.Infof("Cache took %v to sync", endTime.Sub(startTime))

	// load in all our node group listers from our nodegroups
	nodegroupMap := make(map[string]*NodeGroupLister)

	for _, opts := range nodegroups {
		if opts.Name == DefaultNodeGroup {
			nodegroupMap[opts.Name] = NewDefaultNodeGroupLister(allPodLister, allNodeLister, opts)
		} else {
			nodegroupMap[opts.Name] = NewNodeGroupLister(allPodLister, allNodeLister, opts)
		}
	}
	client := Client{
		k8sClient,
		nodegroupMap,
		allPodLister,
		allNodeLister,
	}

	return &client, nil
}
