package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// RunCount is number of times the controller has checked for cluster state
	RunCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "run_count",
		Help: "Number of times the controller has checked for cluster state",
	})
	// NodeGroupNodesUntainted nodes considered by specific node groups that are untainted
	NodeGroupNodesUntainted = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_untainted_nodes",
			Help: "nodes considered by specific node groups that are untainted",
		},
		[]string{"node_group"},
	)
	// NodeGroupNodesTainted nodes considered by specific node groups that are tainted
	NodeGroupNodesTainted = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_tainted_nodes",
			Help: "nodes considered by specific node groups that are tainted",
		},
		[]string{"node_group"},
	)
	// NodeGroupNodes nodes considered by specific node groups
	NodeGroupNodesCordoned = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_cordoned_nodes",
			Help: "nodes considered by specific node groups that are cordoned",
		},
		[]string{"node_group"},
	)
	// NodeGroupNodes nodes considered by specific node groups
	NodeGroupNodes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_nodes",
			Help: "nodes considered by specific node groups",
		},
		[]string{"node_group"},
	)
	// NodeGroupPods pods considered by specific node groups
	NodeGroupPods = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_pods",
			Help: "pods considered by specific node groups",
		},
		[]string{"node_group"},
	)
	// NodeGroupsMemPercent percentage of util of memory
	NodeGroupsMemPercent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_mem_percent",
			Help: "percentage of util of memory",
		},
		[]string{"node_group"},
	)
	// NodeGroupsCPUPercent percentage of util of cpu
	NodeGroupsCPUPercent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_cpu_percent",
			Help: "percentage of util of cpu",
		},
		[]string{"node_group"},
	)
	// NodeGroupMemRequest milli value of node request mem
	NodeGroupMemRequest = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_mem_request",
			Help: "milli value of node request mem",
		},
		[]string{"node_group"},
	)
	// NodeGroupCPURequest milli value of node request cpu
	NodeGroupCPURequest = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_cpu_request",
			Help: "milli value of node request cpu",
		},
		[]string{"node_group"},
	)
	// NodeGroupMemCapacity milli value of node Capacity mem
	NodeGroupMemCapacity = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_mem_capacity",
			Help: "milli value of node Capacity mem",
		},
		[]string{"node_group"},
	)
	// NodeGroupCPUCapacity milli value of node Capacity cpu
	NodeGroupCPUCapacity = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_cpu_capacity",
			Help: "milli value of node capacity cpu",
		},
		[]string{"node_group"},
	)
	// NodeGroupTaintEvent indicates a scale down event
	NodeGroupTaintEvent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_taint_event",
			Help: "indicates a scale down event",
		},
		[]string{"node_group"},
	)
	// NodeGroupUntaintEvent indicates a scale up event
	NodeGroupUntaintEvent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_untaint_event",
			Help: "indicates a scale up event",
		},
		[]string{"node_group"},
	)
	// NodeGroupScaleLock indicates if the nodegroup is locked from scaling
	NodeGroupScaleLock = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "node_group_scale_lock",
			Help: "indicates if the nodegroup is locked from scaling",
		},
		[]string{"node_group"},
	)
	// CloudProviderMinSize indicates the current cloud provider minimum size
	CloudProviderMinSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloud_provider_min_size",
			Help: "current cloud provider minimum size",
		},
		[]string{"cloud_provider", "id"},
	)
	// CloudProviderMaxSize indicates the current cloud provider maximum size
	CloudProviderMaxSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloud_provider_max_size",
			Help: "current cloud provider maximum size",
		},
		[]string{"cloud_provider", "id"},
	)
	// CloudProviderTargetSize indicates the current cloud provider target size
	CloudProviderTargetSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloud_provider_target_size",
			Help: "current cloud provider target size",
		},
		[]string{"cloud_provider", "id"},
	)
	// CloudProviderSize indicates the current cloud provider size
	CloudProviderSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloud_provider_size",
			Help: "current cloud provider size",
		},
		[]string{"cloud_provider", "id"},
	)
)

func init() {
	prometheus.MustRegister(RunCount)
	prometheus.MustRegister(NodeGroupNodes)
	prometheus.MustRegister(NodeGroupNodesCordoned)
	prometheus.MustRegister(NodeGroupNodesUntainted)
	prometheus.MustRegister(NodeGroupNodesTainted)
	prometheus.MustRegister(NodeGroupPods)
	prometheus.MustRegister(NodeGroupsMemPercent)
	prometheus.MustRegister(NodeGroupsCPUPercent)
	prometheus.MustRegister(NodeGroupCPURequest)
	prometheus.MustRegister(NodeGroupMemRequest)
	prometheus.MustRegister(NodeGroupCPUCapacity)
	prometheus.MustRegister(NodeGroupMemCapacity)
	prometheus.MustRegister(NodeGroupTaintEvent)
	prometheus.MustRegister(NodeGroupUntaintEvent)
	prometheus.MustRegister(NodeGroupScaleLock)
	prometheus.MustRegister(CloudProviderMinSize)
	prometheus.MustRegister(CloudProviderMaxSize)
	prometheus.MustRegister(CloudProviderTargetSize)
	prometheus.MustRegister(CloudProviderSize)
}

// Start starts the metrics endpoint on a new thread
func Start(addr string) {
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(addr, nil)
}
