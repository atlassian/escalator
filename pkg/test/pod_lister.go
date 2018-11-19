package test

import (
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func NewTestPodWatcher(pods []*v1.Pod, opts PodListerOptions) listerv1.PodLister {
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	for _, pod := range pods {
		store.Add(pod)
	}
	return &podLister{store, opts}
}

type PodListerOptions struct {
	ReturnErrorOnList bool
}

type podLister struct {
	store cache.Store
	opts  PodListerOptions
}

func (lister *podLister) List(selector labels.Selector) (ret []*v1.Pod, err error) {
	if lister.opts.ReturnErrorOnList {
		return ret, errors.New("unable to list pods")
	}
	err = cache.ListAll(lister.store, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.Pod))
	})
	return ret, err
}

func (lister *podLister) Pods(namespace string) listerv1.PodNamespaceLister {
	return nil
}
