package k8s

import (
	apiV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// DeleteNode deletes a single node from Kubernetes
func DeleteNode(node *apiV1.Node, client kubernetes.Interface) error {
	deleteOptions := &v1.DeleteOptions{}
	return client.CoreV1().Nodes().Delete(node.Name, deleteOptions)
}

// DeleteNodes deletes multiple nodes from Kubernetes
func DeleteNodes(nodes []*apiV1.Node, client kubernetes.Interface) error {
	for _, node := range nodes {
		err := DeleteNode(node, client)
		if err != nil {
			return err
		}
	}
	return nil
}
