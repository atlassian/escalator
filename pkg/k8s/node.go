package k8s

import (
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeleteNode deletes a single node from Kubernetes
func DeleteNode(node *v1.Node, client kubernetes.Interface) error {
	deleteOptions := &v12.DeleteOptions{}
	return client.CoreV1().Nodes().Delete(node.Name, deleteOptions)
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
