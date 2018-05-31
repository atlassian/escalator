package controller

import "k8s.io/api/core/v1"

// nodeIndexBundle bundles an original index to a node so that it can be tracked during sorting
type nodeIndexBundle struct {
	node  *v1.Node
	index int
	pods  []*v1.Pod
}

// nodesByOldestCreationTime Sort functions for sorting by creation time
type nodesByOldestCreationTime []nodeIndexBundle

func (n nodesByOldestCreationTime) Len() int {
	return len(n)
}

func (n nodesByOldestCreationTime) Less(i, j int) bool {
	return n[i].node.CreationTimestamp.Before(&n[j].node.CreationTimestamp)
}

func (n nodesByOldestCreationTime) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

// nodesByNewestCreationTime Sort functions for sorting by creation time
type nodesByNewestCreationTime []nodeIndexBundle

func (n nodesByNewestCreationTime) Len() int {
	return len(n)
}

func (n nodesByNewestCreationTime) Less(i, j int) bool {
	return n[j].node.CreationTimestamp.Before(&n[i].node.CreationTimestamp)
}

func (n nodesByNewestCreationTime) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

// nodesByPodsToRescheduleLeast Sort functions for sorting by the amount of pods to reschedule on a node
type nodesByPodsToRescheduleLeast []nodeIndexBundle

func (n nodesByPodsToRescheduleLeast) Len() int {
	return len(n)
}

func (n nodesByPodsToRescheduleLeast) Less(i, j int) bool {
	return len(n[i].pods) < len(n[j].pods)
}

func (n nodesByPodsToRescheduleLeast) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}
