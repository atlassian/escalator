package cloudprovider

import (
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
)

// CloudProvider contains configuration info and functions for interacting with
// cloud provider (GCE, AWS, etc).
type CloudProvider interface {
	// Name returns name of the cloud provider.
	Name() string

	// NodeGroups returns all node groups configured for this cloud provider.
	NodeGroups() []NodeGroup

	// GetNodeGroup gets the node group from the cloud provider. Returns if it exists or not
	GetNodeGroup(string) (NodeGroup, bool)

	// RegisterNodeGroup adds the nodegroup to the list of nodes groups
	RegisterNodeGroups(...NodeGroupConfig) error

	// Refresh is called before every main loop and can be used to dynamically update cloud provider state.
	// In particular the list of node groups returned by NodeGroups can change as a result of CloudProvider.Refresh().
	Refresh() error

	// GetInstance returns a cloud provider Instance for the provided node. Contains the cloud provider specific
	// instance for cloud provider specific results.
	GetInstance(node *v1.Node) (Instance, error)
}

// Instance contains convenience functions for extracting common information from CP instances
type Instance interface {
	// InstantiationTime gets the time the resource was instantiated
	InstantiationTime() time.Time

	// ID gets the cloud provider resource identifier
	ID() string
}

// NodeGroup contains configuration info and functions to control a set
// of nodes that have the same capacity and set of labels.
type NodeGroup interface {
	// Implements stringer returns a string containing all information regarding this node group.
	fmt.Stringer

	// ID returns an unique identifier of the node group.
	ID() string

	// Name returns the name of the node group for this cloud provider node group.
	Name() string

	// MinSize returns minimum size of the node group.
	MinSize() int64

	// MaxSize returns maximum size of the node group.
	MaxSize() int64

	// TargetSize returns the current target size of the node group. It is possible that the
	// number of nodes in Kubernetes is different at the moment but should be equal
	// to Size() once everything stabilizes (new nodes finish startup and registration or
	// removed nodes are deleted completely).
	TargetSize() int64

	// Size is the number of instances in the nodegroup at the current time
	Size() int64

	// IncreaseSize increases the size of the node group. To delete a node you need
	// to explicitly name it and use DeleteNode. This function should wait until
	// node group size is updated.
	IncreaseSize(delta int64) error

	// Belongs determines if the node belongs in the current node group
	Belongs(*v1.Node) bool

	// DeleteNodes deletes nodes from this node group. Error is returned either on
	// failure or if the given node doesn't belong to this node group. This function
	// should wait until node group size is updated.
	DeleteNodes(...*v1.Node) error

	// DecreaseTargetSize decreases the target size of the node group. This function
	// doesn't permit to delete any existing node and can be used only to reduce the
	// request for new nodes that have not been yet fulfilled. Delta should be negative.
	// It is assumed that cloud provider will not delete the existing nodes when there
	// is an option to just decrease the target.
	DecreaseTargetSize(delta int64) error

	// Nodes returns a list of all nodes that belong to this node group.
	Nodes() []string
}

// Builder interface provides a method to build a cloud provider
type Builder interface {
	Build() (CloudProvider, error)
}

// BuildOpts providers all options to create your cloud provider
type BuildOpts struct {
	ProviderID       string
	NodeGroupConfigs []NodeGroupConfig
}

// NodeGroupConfig contains the configuration for a node group
type NodeGroupConfig struct {
	Name      string
	GroupID   string
	AWSConfig AWSNodeGroupConfig
}

// AWSNodeGroupConfig contains the AWS cloud provider specific configuration
// for a node group
type AWSNodeGroupConfig struct {
	LaunchTemplateID          string
	LaunchTemplateVersion     string
	FleetInstanceReadyTimeout time.Duration
	Lifecycle                 string
	InstanceTypeOverrides     []string
}
