package k8s

import (
	"testing"
	"time"

	apiv1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"

	"strconv"

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
		return true, nil, apiErrors.NewNotFound(apiv1.Resource("node"), get.GetName())
	})
	fakeClient.Fake.AddReactor("update", "nodes", func(action core.Action) (bool, runtime.Object, error) {
		update := action.(core.UpdateAction)
		obj := update.GetObject().(*apiv1.Node)
		updatedNodes <- obj.Name
		return true, obj, nil
	})
	return fakeClient, updatedNodes
}

// getStringFromChan gets the first string from a channel
// if it is empty, return "nothing returned"
func getStringFromChan(c chan string) string {
	if len(c) > 0 {
		return <-c
	}
	return "nothing returned"
}

func TestAddToBeRemovedTaint(t *testing.T) {
	node := test.BuildTestNode(test.NodeOpts{})
	fakeClient, updatedNodes := buildFakeClientAndUpdateChannel(node)
	updated, err := AddToBeRemovedTaint(node, fakeClient, "NoExecute")

	assert.NoError(t, err)
	assert.Equal(t, updated.Name, getStringFromChan(updatedNodes))
	taint, ok := GetToBeRemovedTaint(updated)
	assert.True(t, ok)
	assert.Equal(t, apiv1.TaintEffectNoExecute, taint.Effect)
}

func TestAddToBeRemovedTaint_DefaultNoScheduleTaintOnEmptyObject(t *testing.T) {
	node := test.BuildTestNode(test.NodeOpts{})
	fakeClient, updatedNodes := buildFakeClientAndUpdateChannel(node)
	updated, err := AddToBeRemovedTaint(node, fakeClient, apiv1.TaintEffect(""))

	assert.NoError(t, err)
	assert.Equal(t, updated.Name, getStringFromChan(updatedNodes))
	taint, ok := GetToBeRemovedTaint(updated)
	assert.True(t, ok)
	assert.NotEmpty(t, taint)
	assert.Equal(t, apiv1.TaintEffectNoSchedule, taint.Effect)
}

func TestAddToBeRemovedTaint_DefaultNoScheduleTaintOnEmptyString(t *testing.T) {
	node := test.BuildTestNode(test.NodeOpts{})
	fakeClient, updatedNodes := buildFakeClientAndUpdateChannel(node)
	updated, err := AddToBeRemovedTaint(node, fakeClient, "")

	assert.NoError(t, err)
	assert.Equal(t, updated.Name, getStringFromChan(updatedNodes))
	taint, ok := GetToBeRemovedTaint(updated)
	assert.True(t, ok)
	assert.Equal(t, apiv1.TaintEffectNoSchedule, taint.Effect)
}

func TestAddToBeRemovedTaint_AlreadyExists(t *testing.T) {
	node := test.BuildTestNode(test.NodeOpts{})
	fakeClient, updatedNodes := buildFakeClientAndUpdateChannel(node)

	// Add the taint
	updated, err := AddToBeRemovedTaint(node, fakeClient, "NoSchedule")
	assert.NoError(t, err)
	assert.Equal(t, updated.Name, getStringFromChan(updatedNodes))

	// Remake the fake client with the updated node
	fakeClient, updatedNodes = buildFakeClientAndUpdateChannel(updated)

	// Add the taint again on the updated node
	_, err = AddToBeRemovedTaint(updated, fakeClient, "NoSchedule")
	assert.NoError(t, err)
	// Ensure the taint is not added again
	assert.Equal(t, "nothing returned", getStringFromChan(updatedNodes))
}

func TestGetToBeRemovedTaint(t *testing.T) {
	node := test.BuildTestNode(test.NodeOpts{})
	fakeClient, updatedNodes := buildFakeClientAndUpdateChannel(node)
	updated, err := AddToBeRemovedTaint(node, fakeClient, "NoExecute")

	assert.NoError(t, err)
	assert.Equal(t, updated.Name, getStringFromChan(updatedNodes))
	taint, ok := GetToBeRemovedTaint(updated)
	assert.True(t, ok)
	assert.Equal(t, ToBeRemovedByAutoscalerKey, taint.Key)
	assert.Equal(t, apiv1.TaintEffectNoExecute, taint.Effect)
}

func TestGetToBeRemovedTime(t *testing.T) {
	node := test.BuildTestNode(test.NodeOpts{})

	// Get the time before adding the taint to the node
	val, err := GetToBeRemovedTime(node)
	assert.Nil(t, val)
	assert.Nil(t, err)

	// Create a fake client
	fakeClient, updatedNodes := buildFakeClientAndUpdateChannel(node)

	// Add the taint to the node
	updated, err := AddToBeRemovedTaint(node, fakeClient, "NoSchedule")
	assert.NoError(t, err)
	assert.Equal(t, updated.Name, getStringFromChan(updatedNodes))
	_, ok := GetToBeRemovedTaint(updated)
	assert.True(t, ok)

	// Get the taint to be removed time
	val, err = GetToBeRemovedTime(updated)
	assert.NoError(t, err)
	assert.True(t, time.Since(*val) < 10*time.Second)
}

func TestGetToBeRemovedTime_InvalidValue(t *testing.T) {
	node := test.BuildTestNode(test.NodeOpts{})

	// Add the invalid taint
	node.Spec.Taints = append(node.Spec.Taints, apiv1.Taint{
		Key:    ToBeRemovedByAutoscalerKey,
		Value:  "invalid-value", // invalid value, should be current time as a unix timestamp
		Effect: apiv1.TaintEffectNoSchedule,
	})

	val, err := GetToBeRemovedTime(node)
	assert.Nil(t, val)
	assert.IsType(t, &strconv.NumError{}, err)
}

func TestDeleteToBeRemovedTaint(t *testing.T) {
	node := test.BuildTestNode(test.NodeOpts{})
	fakeClient, updatedNodes := buildFakeClientAndUpdateChannel(node)

	updated, err := AddToBeRemovedTaint(node, fakeClient, "NoSchedule")
	assert.NoError(t, err)
	assert.Equal(t, updated.Name, getStringFromChan(updatedNodes))

	updated, err = DeleteToBeRemovedTaint(node, fakeClient)
	assert.NoError(t, err)
	assert.Equal(t, updated.Name, getStringFromChan(updatedNodes))
	_, ok := GetToBeRemovedTaint(updated)
	assert.False(t, ok)
}
