package k8s

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Cordon cordons the node
// returns the latest update of the node
func Cordon(node *apiv1.Node, client kubernetes.Interface) (*apiv1.Node, error) {
	// fetch the latest version of the node to avoid conflict
	updatedNode, err := client.CoreV1().Nodes().Get(node.Name, metav1.GetOptions{})
	if err != nil || updatedNode == nil {
		return node, fmt.Errorf("failed to get node %v: %v", node.Name, err)
	}

	// check if already cordoned
	if node.Spec.Unschedulable {
		return updatedNode, nil
	}

	node.Spec.Unschedulable = true

	updatedNodeWithCordon, err := client.CoreV1().Nodes().Update(updatedNode)
	if err != nil || updatedNodeWithCordon == nil {
		return updatedNode, fmt.Errorf("failed to update node %v after adding cordon: %v", updatedNode.Name, err)
	}

	log.Infof("Successfully added cordon on node %v", updatedNodeWithCordon.Name)
	IncrementTaintCount()
	return updatedNodeWithCordon, nil
}
