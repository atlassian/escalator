package k8s

import (
	"testing"
	"time"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"

	"github.com/atlassian/escalator/pkg/test"
	"github.com/stretchr/testify/assert"
)

// Borrowed from: https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/utils/deletetaint/delete_test.go
func buildFakeClientAndUpdateChannel(node *apiv1.Node) (*fake.Clientset, chan string) {
	fakeClient := &fake.Clientset{}
	updatedNodes := make(chan string, 10)
	fakeClient.Fake.AddReactor("get", "nodes", func(action core.Action) (bool, runtime.Object, error) {
		get := action.(core.GetAction)
		if get.GetName() == node.Name {
			return true, node, nil
		}
		return true, nil, errors.NewNotFound(apiv1.Resource("node"), get.GetName())
	})
	fakeClient.Fake.AddReactor("update", "nodes", func(action core.Action) (bool, runtime.Object, error) {
		update := action.(core.UpdateAction)
		obj := update.GetObject().(*apiv1.Node)
		updatedNodes <- obj.Name
		return true, obj, nil
	})
	return fakeClient, updatedNodes
}

// Borrowed from: https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/utils/deletetaint/delete_test.go
func getStringFromChan(c chan string) string {
	select {
	case val := <-c:
		return val
	case <-time.After(time.Second * 10):
		return "Nothing returned"
	}
}

func TestAddToBeRemovedTaint(t *testing.T) {
	node := test.BuildTestNode(test.NodeOpts{
		Name: "node",
		CPU:  1000,
		Mem:  1000,
	})
	fakeClient, updatedNodes := buildFakeClientAndUpdateChannel(node)
	updated, err := AddToBeRemovedTaint(node, fakeClient)

	assert.NoError(t, err)
	assert.Equal(t, updated.Name, getStringFromChan(updatedNodes))
	_, ok := GetToBeRemovedTaint(updated)
	assert.True(t, ok)
}

func TestGetToBeRemovedTaint(t *testing.T) {
	node := test.BuildTestNode(test.NodeOpts{
		Name: "node",
		CPU:  1000,
		Mem:  1000,
	})
	fakeClient, updatedNodes := buildFakeClientAndUpdateChannel(node)
	updated, err := AddToBeRemovedTaint(node, fakeClient)

	assert.NoError(t, err)
	assert.Equal(t, updated.Name, getStringFromChan(updatedNodes))
	taint, ok := GetToBeRemovedTaint(updated)
	assert.True(t, ok)
	assert.Equal(t, ToBeRemovedByAutoscalerKey, taint.Key)
	assert.Equal(t, apiv1.TaintEffectNoSchedule, taint.Effect)
}

func TestGetToBeRemovedTime(t *testing.T) {
	node := test.BuildTestNode(test.NodeOpts{
		Name: "node",
		CPU:  1000,
		Mem:  1000,
	})
	fakeClient, updatedNodes := buildFakeClientAndUpdateChannel(node)
	updated, err := AddToBeRemovedTaint(node, fakeClient)

	assert.NoError(t, err)
	assert.Equal(t, updated.Name, getStringFromChan(updatedNodes))
	_, ok := GetToBeRemovedTaint(updated)
	assert.True(t, ok)

	val, err := GetToBeRemovedTime(updated)
	assert.NoError(t, err)
	assert.True(t, time.Now().Sub(*val) < 10*time.Second)
}

func TestDeleteToBeRemovedTaint(t *testing.T) {
	node := test.BuildTestNode(test.NodeOpts{
		Name: "node",
		CPU:  1000,
		Mem:  1000,
	})
	fakeClient, updatedNodes := buildFakeClientAndUpdateChannel(node)

	updated, err := AddToBeRemovedTaint(node, fakeClient)
	assert.NoError(t, err)
	assert.Equal(t, updated.Name, getStringFromChan(updatedNodes))

	updated, err = DeleteToBeRemovedTaint(node, fakeClient)
	assert.NoError(t, err)
	assert.Equal(t, updated.Name, getStringFromChan(updatedNodes))
	_, ok := GetToBeRemovedTaint(updated)
	assert.False(t, ok)
}
