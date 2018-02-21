package k8s

import "k8s.io/api/core/v1"

// NodeEmpty returns if the node is empty of pods, except for daemonsets
func NodeEmpty(node *v1.Node) bool {
	return false
}
