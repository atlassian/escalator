# Metrics

Escalator exposes metrics of scaling, requests, capacity, utilisation, nodes and pods at the `/metrics` endpoint. 
These metrics can be scraped by a monitoring system such as [Prometheus](https://prometheus.io/).

It is highly recommended to collect the metrics exposed by Escalator, as it can provide helpful insight into how
it is operating as well as when you need to debug it's operation.

Below is an example of a [Grafana](https://grafana.com/) dashboard that provides insight into the overall utilisation 
of the node group, as well as the total tainted/untainted nodes.

![Metrics Dashboard](./metrics-dashboard.png)

## Exposed Metrics

These are the metrics that Escalator exposes, and are subject to change:

 - **`run_count`**: Number of times the controller has checked for cluster state
 - **`node_group_untainted_nodes`**: nodes considered by specific node groups that are untainted
 - **`node_group_tainted_nodes`**: nodes considered by specific node groups that are tainted
 - **`node_group_cordoned_nodes`**: nodes considered by specific node groups that are cordoned
 - **`node_group_nodes`**: nodes considered by specific node groups
 - **`node_group_pods`**: pods considered by specific node groups
 - **`node_group_mem_percent`**: percentage of util of memory
 - **`node_group_cpu_percent`**: percentage of util of cpu
 - **`node_group_mem_request`**: milli value of node request mem
 - **`node_group_cpu_request`**: milli value of node request cpu
 - **`node_group_mem_capacity`**: milli value of node Capacity mem
 - **`node_group_cpu_capacity`**: milli value of node capacity cpu
 - **`node_group_taint_event`**: indicates a scale down event
 - **`node_group_untaint_event`**: indicates a scale up event
 - **`node_group_scale_lock`**: indicates if the nodegroup is locked from scaling
 
 ## Recommendations
 
 It is highly recommended to monitor and graph the two utilisation metrics 
 (`node_group_mem_percent` and `node_group_cpu_percent`) as this will let you see the utilisation that Escalator
 calculates. Ideally these values should stay below your scale up threshold.
