# Advanced Configuration

## Threshold Configuration

Scaling thresholds are configured using the following main nodegroup configuration options:

```yaml
taint_upper_capacity_threshold_percent: 40
taint_lower_capacity_threshold_percent: 10
scale_up_threshold_percent: 70
```
Full configuration options can be found in [Node Group Configuration](./nodegroup.md).

Through the configuration of these three options we can tell Escalator when we would like it to scale up or down the
cluster or when we want it to do nothing.

- `scale_up_threshold_percent` defines the threshold for scaling up.
- `taint_upper_capacity_threshold_percent` and `taint_lower_capacity_threshold_percent` defines the thresholds for
tainting which will lead to scaling down.
- If the cluster utilisation falls between `taint_upper_capacity_threshold_percent` and `scale_up_threshold_percent`,
Escalator will do nothing.

Given the above threshold values, Escalator will do the following:

- Utilisation: 110% = **scale up** (exceeded `scale_up_threshold_percent`)
- Utilisation: 75% = **scale up** (exceeded `scale_up_threshold_percent`)
- Utilisation: 70% = **scale up** (exceeded `scale_up_threshold_percent`)
- Utilisation: 50% = **do nothing**
- Utilisation: 40% = **do nothing** (utilisation has to be lower than the threshold for it to trigger)
- Utilisation: 38% = **scale down slowly** (below `taint_upper_capacity_threshold_percent` 
but above `taint_lower_capacity_threshold_percent` so only scale down slowly)
- Utilisation: 10% = **scale down slowly** (below `taint_upper_capacity_threshold_percent` 
but above `taint_lower_capacity_threshold_percent` so only scale down slowly)
- Utilisation: 9% = **scale down quickly** (below `taint_lower_capacity_threshold_percent`)
- Utilisation: 0% = **scale down quickly** (below `taint_lower_capacity_threshold_percent`)

## Slack Capacity Configuration

Slack capacity is configured through the `scale_up_threshold_percent` option. If this option is set to **70** for 
example, whenever the node group utilisation reaches or exceeds **70%**, a scale up will occur (of course, it will
try to untaint any existing nodes first to meet the requirement). Escalator will try and
keep the node group utilisation below **70%** and thus there will be a "slack capacity" of **30%**.

Bear in mind, this **30%** "slack capacity" is NOT that exact amount of allocatable resources reserved. In reality,
it's less than that reserved value. Because escalator doesn't count any 
[Daemonset](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/) pod or 
[Static](https://kubernetes.io/docs/tasks/administer-cluster/static-pod/) pod when calculating that usage percentage.
So you need to deduct all existing daemonset/static pod requests when calculating that `scale_up_threshold_percent`
to get more accurate amount of slack capacity reserved.

If you want to tune slack capacity from cost point of view, you should also look at `taint_upper_capacity_threshold_percent`
and `taint_lower_capacity_threshold_percent`. Because those two parameters will extend average slack capacity a bit more
if you tune them based on your workload while `scale_up_threshold_percent` is trying to keep the minimum slack capacity for
your cluster.

To completely eliminate slack capacity, `scale_up_threshold_percent` can be set to **100**, which will mean that
Escalator will only scale up the node group when the utilisation reaches or exceeds **100%**.

It is recommended to have some slack capacity in the event that there is a sudden spike of new pods to allow for
Escalator time to increase the node group size before pods cannot be scheduled.
