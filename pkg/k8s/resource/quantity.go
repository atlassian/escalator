package resource

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

func NewMemoryQuantity(value int64) *resource.Quantity {
	return resource.NewQuantity(value, resource.BinarySI)
}

func NewCPUQuantity(value int64) *resource.Quantity {
	return resource.NewMilliQuantity(value, resource.DecimalSI)
}

func NewPodQuantity(value int64) *resource.Quantity {
	return resource.NewQuantity(value, resource.DecimalSI)
}
