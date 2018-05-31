package k8s

import (
	"github.com/atlassian/escalator/pkg/metrics"
	log "github.com/sirupsen/logrus"
	apiV1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	EvictionKind        = "Eviction"
	EvictionSubresource = "pods/eviction"
)

// EvictPod evicts a single pod from Kubernetes
func EvictPod(pod *apiV1.Pod, apiVersion string, client kubernetes.Interface, nodeGroupName string) error {
	log.Warnf("Evicting pod: %v/%v", pod.Namespace, pod.Name)
	metrics.NodeGroupPodsEvicted.WithLabelValues(nodeGroupName).Inc()
	return client.CoreV1().Pods(pod.Namespace).Evict(&v1beta1.Eviction{
		TypeMeta: v1.TypeMeta{
			APIVersion: apiVersion,
			Kind:       EvictionKind,
		},
		ObjectMeta:    pod.ObjectMeta,
		DeleteOptions: &v1.DeleteOptions{},
	})
}

// EvictPods evicts multiple pods from Kubernetes
func EvictPods(pods []*apiV1.Pod, apiVersion string, client kubernetes.Interface, nodeGroupName string) error {
	for _, pod := range pods {
		err := EvictPod(pod, apiVersion, client, nodeGroupName)
		if err != nil {
			return err
		}
	}
	return nil
}

// DeletePod deletes a single Pod from Kubernetes
func DeletePod(pod *apiV1.Pod, client kubernetes.Interface, nodeGroupName string) error {
	deleteOptions := &v1.DeleteOptions{}
	log.Warnf("Deleting pod: %v/%v", pod.Namespace, pod.Name)
	metrics.NodeGroupPodsDeleted.WithLabelValues(nodeGroupName).Inc()
	return client.CoreV1().Pods(pod.Namespace).Delete(pod.Name, deleteOptions)
}

// DeletePods deletes multiple Pods from Kubernetes
func DeletePods(pods []*apiV1.Pod, client kubernetes.Interface, nodeGroupName string) error {
	for _, pod := range pods {
		err := DeletePod(pod, client, nodeGroupName)
		if err != nil {
			return err
		}
	}
	return nil
}

// SupportEviction uses Discovery API to find out if the API server supports the eviction subresource
// If there is support, it will return its groupVersion; Otherwise, it will return ""
func SupportEviction(clientset kubernetes.Interface) (string, error) {
	discoveryClient := clientset.Discovery()
	groupList, err := discoveryClient.ServerGroups()
	if err != nil {
		return "", err
	}
	foundPolicyGroup := false
	var policyGroupVersion string
	for _, group := range groupList.Groups {
		if group.Name == "policy" {
			foundPolicyGroup = true
			policyGroupVersion = group.PreferredVersion.GroupVersion
			break
		}
	}
	if !foundPolicyGroup {
		return "", nil
	}
	resourceList, err := discoveryClient.ServerResourcesForGroupVersion("v1")
	if err != nil {
		return "", err
	}
	for _, resource := range resourceList.APIResources {
		if resource.Name == EvictionSubresource && resource.Kind == EvictionKind {
			return policyGroupVersion, nil
		}
	}
	return "", nil
}
