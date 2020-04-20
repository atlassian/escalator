package k8s

import (
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

// CreateNodeNameToInfoMap creates a map of NodeInfo which maps node names to nodes and pods to nodes
// From K8s cluster-autoscaler. Based off the old scheduler cache.NodeInfo
func CreateNodeNameToInfoMap(pods []*v1.Pod, nodes []*v1.Node) map[string]*NodeInfo {
	nodeNameToNodeInfo := make(map[string]*NodeInfo)
	for _, pod := range pods {
		nodeName := pod.Spec.NodeName
		if _, ok := nodeNameToNodeInfo[nodeName]; !ok {
			nodeNameToNodeInfo[nodeName] = NewNodeInfo()
		}
		nodeNameToNodeInfo[nodeName].AddPod(pod)
	}

	for _, node := range nodes {
		if _, ok := nodeNameToNodeInfo[node.Name]; !ok {
			nodeNameToNodeInfo[node.Name] = NewNodeInfo()
		}
		nodeNameToNodeInfo[node.Name].SetNode(node)
	}

	// Some pods may be out of sync with node lists. Removing incomplete node infos.
	keysToRemove := make([]string, 0)
	for key, nodeInfo := range nodeNameToNodeInfo {
		if nodeInfo.Node() == nil {
			keysToRemove = append(keysToRemove, key)
		}
	}
	for _, key := range keysToRemove {
		delete(nodeNameToNodeInfo, key)
	}

	return nodeNameToNodeInfo
}

// NodeEmpty returns if the node is empty of pods, except for daemonsets
func NodeEmpty(node *v1.Node, nodeInfoMap map[string]*NodeInfo) bool {
	nodePodsRemaining, ok := NodePodsRemaining(node, nodeInfoMap)
	return ok && nodePodsRemaining == 0
}

// NodePodsRemaining returns the number of pods on the node, except for daemonset pods
func NodePodsRemaining(node *v1.Node, nodeInfoMap map[string]*NodeInfo) (int, bool) {
	nodeInfo, ok := nodeInfoMap[node.Name]
	if !ok {
		log.Warningf("could not find node %v in the nodeinfo map", node.Name)
		return 0, false
	}

	// check all the pods and make sure they're daemonsets
	// otherwise there are sacred pods still on the node
	pods := 0
	for _, pod := range nodeInfo.Pods() {
		if !PodIsDaemonSet(pod) {
			pods++
		}
	}

	return pods, true
}
