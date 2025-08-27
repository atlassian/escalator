package k8s

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// DeleteNode deletes a single node from Kubernetes
func DeleteNode(node *v1.Node, client kubernetes.Interface) error {
	deleteOptions := metav1.DeleteOptions{}
	return client.CoreV1().Nodes().Delete(context.TODO(), node.Name, deleteOptions)
}

// DeleteNodes deletes multiple nodes from Kubernetes
func DeleteNodes(nodes []*v1.Node, client kubernetes.Interface) error {
	for _, node := range nodes {
		err := DeleteNode(node, client)
		if err != nil {
			return err
		}
	}
	return nil
}

// IsNodeUnhealthy returns true if the node is not ready by the amount of time
// allowed.
func IsNodeUnhealthy(node *v1.Node, gracePeriod time.Duration) bool {
	// If a node is cordoned then do not consider it unhealthy
	if node.Spec.Unschedulable {
		return false
	}

	// If the grace period expiry time is in the future then the instance is not
	// deemed to be unhealthy even if it is not ready.
	if node.CreationTimestamp.Add(gracePeriod).After(time.Now()) {
		return false
	}

	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady {
			// ConditionTrue shows whether the node is ready, anything else
			// and we can assume the node is not.
			return condition.Status != v1.ConditionTrue
		}
	}

	return true
}
