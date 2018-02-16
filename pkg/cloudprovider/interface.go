package cloudprovider

import "k8s.io/api/core/v1"

type CloudProvider interface {
	Name() string

	NodeGroups() []NodeGroup

	Refresh() error

	Stop() error
}

type NodeGroup interface {
	ID() string

	Debug() string

	MinSize() int

	MaxSize() int

	TargetSize() (int, error)

	IncreaseSize(delta int) error

	DeleteNodes(...*v1.Node) error

	DecreaseTargetSize(delta int) error

	Nodes() ([]string, error)
}
