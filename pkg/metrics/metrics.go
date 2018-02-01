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
			Name: "node_groups_mem_percent",
			Help: "percentage of util of memory",
		},
		[]string{"node_group"},
	)
	// NodeGroupsCPUPercent percentage of util of cpu
	NodeGroupsCPUPercent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_groups_cpu_percent",
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
	// NodeGroupTaintEvent indicates a scale event
	NodeGroupTaintEvent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_taint_event",
			Help: "indicates a scale event",
		},
		[]string{"customer"},
	)
)

func init() {
	prometheus.MustRegister(RunCount)
	prometheus.MustRegister(NodeGroupNodes)
	prometheus.MustRegister(NodeGroupPods)
	prometheus.MustRegister(NodeGroupsMemPercent)
	prometheus.MustRegister(NodeGroupsCPUPercent)
	prometheus.MustRegister(NodeGroupCPURequest)
	prometheus.MustRegister(NodeGroupMemRequest)
	prometheus.MustRegister(NodeGroupCPUCapacity)
	prometheus.MustRegister(NodeGroupMemCapacity)
	prometheus.MustRegister(NodeGroupTaintEvent)
}

// Start starts the metrics endpoint on a new thread
func Start(addr string) {
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(addr, nil)
}
