package test

import (
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func NewTestNodeWatcher(nodes []*v1.Node, opts NodeListerOptions) listerv1.NodeLister {
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	for _, node := range nodes {
		store.Add(node)
	}
	return &nodeLister{store, opts}
}

type NodeListerOptions struct {
	ReturnErrorOnList bool
}

type nodeLister struct {
	store cache.Store
	opts  NodeListerOptions
}

func (lister *nodeLister) List(selector labels.Selector) (ret []*v1.Node, err error) {
	if lister.opts.ReturnErrorOnList {
		return ret, errors.New("unable to list nodes")
	}
	err = cache.ListAll(lister.store, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.Node))
	})
	return ret, err
}

func (lister *nodeLister) Get(name string) (*v1.Node, error) {
	return nil, nil
}

func (lister *nodeLister) ListWithPredicate(predicate listerv1.NodeConditionPredicate) ([]*v1.Node, error) {
	return nil, nil
}
