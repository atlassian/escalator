package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const NAMESPACE = "escalator"
var (
	// RunCount is number of times the controller has checked for cluster state
	RunCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "run_count",
		Namespace: NAMESPACE,
		Help: "Number of times the controller has checked for cluster state",
	})
	// NodeGroupNodesUntainted nodes considered by specific node groups that are untainted
	NodeGroupNodesUntainted = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_untainted_nodes",
			Namespace: NAMESPACE,
			Help: "nodes considered by specific node groups that are untainted",
		},
		[]string{"node_group"},
	)
	// NodeGroupNodesTainted nodes considered by specific node groups that are tainted
	NodeGroupNodesTainted = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_tainted_nodes",
			Namespace: NAMESPACE,
			Help: "nodes considered by specific node groups that are tainted",
		},
		[]string{"node_group"},
	)
	// NodeGroupNodes nodes considered by specific node groups
	NodeGroupNodesCordoned = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_cordoned_nodes",
			Namespace: NAMESPACE,
			Help: "nodes considered by specific node groups that are cordoned",
		},
		[]string{"node_group"},
	)
	// NodeGroupNodes nodes considered by specific node groups
	NodeGroupNodes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_nodes",
			Namespace: NAMESPACE,
			Help: "nodes considered by specific node groups",
		},
		[]string{"node_group"},
	)
	// NodeGroupPods pods considered by specific node groups
	NodeGroupPods = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_pods",
			Namespace: NAMESPACE,
			Help: "pods considered by specific node groups",
		},
		[]string{"node_group"},
	)
	// NodeGroupsPodEvicted pods evicted during a scale down
	NodeGroupPodsEvicted = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "node_group_pods_evicted",
			Namespace: NAMESPACE,
			Help: "pods evicted during a scale down",
		},
		[]string{"node_group"},
	)
	// NodeGroupsMemPercent percentage of util of memory
	NodeGroupsMemPercent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_mem_percent",
			Namespace: NAMESPACE,
			Help: "percentage of util of memory",
		},
		[]string{"node_group"},
	)
	// NodeGroupsCPUPercent percentage of util of cpu
	NodeGroupsCPUPercent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_cpu_percent",
			Namespace: NAMESPACE,
			Help: "percentage of util of cpu",
		},
		[]string{"node_group"},
	)
	// NodeGroupMemRequest milli value of node request mem
	NodeGroupMemRequest = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_mem_request",
			Namespace: NAMESPACE,
			Help: "milli value of node request mem",
		},
		[]string{"node_group"},
	)
	// NodeGroupCPURequest milli value of node request cpu
	NodeGroupCPURequest = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_cpu_request",
			Namespace: NAMESPACE,
			Help: "milli value of node request cpu",
		},
		[]string{"node_group"},
	)
	// NodeGroupMemCapacity milli value of node Capacity mem
	NodeGroupMemCapacity = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_mem_capacity",
			Namespace: NAMESPACE,
			Help: "milli value of node Capacity mem",
		},
		[]string{"node_group"},
	)
	// NodeGroupCPUCapacity milli value of node Capacity cpu
	NodeGroupCPUCapacity = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_cpu_capacity",
			Namespace: NAMESPACE,
			Help: "milli value of node capacity cpu",
		},
		[]string{"node_group"},
	)
	// NodeGroupTaintEvent indicates a scale down event
	NodeGroupTaintEvent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_taint_event",
			Namespace: NAMESPACE,
			Help: "indicates a scale down event",
		},
		[]string{"node_group"},
	)
	// NodeGroupUntaintEvent indicates a scale up event
	NodeGroupUntaintEvent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_untaint_event",
			Namespace: NAMESPACE,
			Help: "indicates a scale up event",
		},
		[]string{"node_group"},
	)
	// NodeGroupScaleLock indicates if the nodegroup is locked from scaling
	NodeGroupScaleLock = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "node_group_scale_lock",
			Namespace: NAMESPACE,
			Help: "indicates if the nodegroup is locked from scaling",
		},
		[]string{"node_group"},
	)
	NodeGroupScaleDelta = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_scale_delta",
			Namespace: NAMESPACE,
			Help: "indicates current scale delta",
		},
		[]string{"node_group"},
	)
	// CloudProviderMinSize indicates the current cloud provider minimum size
	CloudProviderMinSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloud_provider_min_size",
			Namespace: NAMESPACE,
			Help: "current cloud provider minimum size",
		},
		[]string{"cloud_provider", "id"},
	)
	// CloudProviderMaxSize indicates the current cloud provider maximum size
	CloudProviderMaxSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloud_provider_max_size",
			Namespace: NAMESPACE,
			Help: "current cloud provider maximum size",
		},
		[]string{"cloud_provider", "id"},
	)
	// CloudProviderTargetSize indicates the current cloud provider target size
	CloudProviderTargetSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloud_provider_target_size",
			Namespace: NAMESPACE,
			Help: "current cloud provider target size",
		},
		[]string{"cloud_provider", "id"},
	)
	// CloudProviderSize indicates the current cloud provider size
	CloudProviderSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloud_provider_size",
			Namespace: NAMESPACE,
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
	prometheus.MustRegister(NodeGroupPodsEvicted)
	prometheus.MustRegister(NodeGroupsMemPercent)
	prometheus.MustRegister(NodeGroupsCPUPercent)
	prometheus.MustRegister(NodeGroupCPURequest)
	prometheus.MustRegister(NodeGroupMemRequest)
	prometheus.MustRegister(NodeGroupCPUCapacity)
	prometheus.MustRegister(NodeGroupMemCapacity)
	prometheus.MustRegister(NodeGroupTaintEvent)
	prometheus.MustRegister(NodeGroupUntaintEvent)
	prometheus.MustRegister(NodeGroupScaleLock)
	prometheus.MustRegister(NodeGroupScaleDelta)
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
