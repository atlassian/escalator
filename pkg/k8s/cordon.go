package k8s

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Cordon sets the cordoned state of the node to cordon boolean
// returns the latest update of the node
func Cordon(cordon bool, node *apiv1.Node, client kubernetes.Interface) (*apiv1.Node, error) {
	// fetch the latest version of the node to avoid conflict
	updatedNode, err := client.CoreV1().Nodes().Get(node.Name, metav1.GetOptions{})
	if err != nil || updatedNode == nil {
		return node, fmt.Errorf("failed to get node %v: %v", node.Name, err)
	}

	// check if already un/cordoned
	if updatedNode.Spec.Unschedulable == cordon {
		log.WithField("cordon", cordon).Warnf("Node %v cordon is already un/cordoned", updatedNode.Name)
		return updatedNode, nil
	}

	updatedNode.Spec.Unschedulable = cordon

	updatedNodeWithCordon, err := client.CoreV1().Nodes().Update(updatedNode)
	if err != nil || updatedNodeWithCordon == nil {
		return updatedNode, fmt.Errorf("failed to update node %v after setting cordon to %v: %v", updatedNode.Name, cordon, err)
	}

	log.WithField("cordon", cordon).Infof("Successfully changed cordon state on node %v", updatedNodeWithCordon.Name)
	return updatedNodeWithCordon, nil
}
