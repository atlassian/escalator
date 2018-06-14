# Node Group Configuration

Configuration of the Node groups that Escalator will monitor is done through a YAML configuration file. This file is
required for Escalator to run. 

The configuration is validated by Escalator on start.

Example `nodegroups_config.yaml` configuration:

```yaml
node_groups:
  - name: "shared"
    label_key: "customer"
    label_value: "shared"
    cloud_provider_group_name: "shared-nodes"
    min_nodes: 1
    max_nodes: 30
    dry_mode: false
    taint_upper_capacity_threshold_percent: 40
    taint_lower_capacity_threshold_percent: 10
    slow_node_removal_rate: 2
    fast_node_removal_rate: 5
    scale_up_threshold_percent: 70
    scale_up_cool_down_period: 2m
    scale_up_cool_down_timeout: 10m
    soft_delete_grace_period: 1m
    hard_delete_grace_period: 10m
```

## Options

### `name`

This is the name of the node group to be displayed in the logs of Escalator when it is running. It doesn't provide
any other benefit other than being useful for debugging a specific nodegroup.

Multiple node groups will need to have unique names.

If you configure the name of the node group to be `default`, it will watch all pods that do not have any affinity or
node selectors specified. More information on the `default` option can be found [here](../pod-node-selectors.md).

### `label_key` and `label_value`

`label_key` and `label_value` is the key-value pair used to select nodes and pods for consideration in the calculations 
for a node group.

**Pod and Node selectors are documented [here](../pod-node-selectors.md).**

### `cloud_provider_group_name`

`cloud_provider_group_name` is the node group in the cloud provider that Escalator will either increase the size
of or terminate nodes in when Escalator scales up or down.

- **AWS:** this is the name of the auto scaling group. More information on AWS deployments can be found 
[here](../deployment/aws/README.md).

### `min_nodes` and `max_nodes`

These are the required hard limits that Escalator will stay within when performing scale up or down activities. If 
there is a case where a scale up or down activity will exceed either of these values, Escalator will clamp/adjust the 
value to the min or max value. For example:

`min_nodes` is configured to be **5**. The current node count is **7**. Escalator needs to scale down and terminate 
**3** nodes. The new current node count after scale down will exceed **5** if we terminate **3** nodes, so Escalator 
will clamp/adjust the scale down amount to only terminate **2** nodes. The new current node count will be **5**.

`max_nodes` is configured to be **20**. The current node count is **18**. Escalator needs to scale up and create 
**3** nodes. The new current node count after scale up will exceed **20** if we create **3** nodes, so Escalator will 
clamp/adjust the scale up amount to create **2** nodes. The new current node count will be **20**.

`min_nodes` and `max_nodes` do not have to match the values in the cloud provider. Note: if you configure Escalator's
`min_nodes` and `max_nodes` to be larger/smaller than the cloud provider's, the cloud provider may still block the scale
activity if the activity exceeds the cloud provider's limits.

### `dry_mode`

This flag allows running a specific node group in dry mode. This will ensure Escalator doesn't taint, cordon or modify
the node group, but just logs out the actions it would perform. This is helpful in understanding what
Escalator would do in specific scenarios.

Note: this flag is overridden by the `--drymode` command line flag.

### `taint_upper_capacity_threshold_percent`

This option defines the threshold at which Escalator will slowly start tainting nodes. The slow tainting will only occur
when the utilisation of the node group goes below this value. For example:

If the node group utilisation is **38%**, and `taint_upper_capacity_threshold_percent` is configured as **40**,
Escalator will taint nodes at the rate defined in `slow_node_removal_rate`.

### `taint_lower_capacity_threshold_percent`

This option defines the threshold at which Escalator will quickly/fast start tainting nodes. The fast tainting will only
occur when the utilisation of the node group goes below this value. For example:

If the node group utilisation is **5%**, and `taint_upper_capacity_threshold_percent` is configured as **10**,
Escalator will taint nodes at the rate defined in `fast_node_removal_rate`.

### `slow_node_removal_rate`

The amount of nodes to taint whenever the node group utilisation goes below the 
`taint_upper_capacity_threshold_percent` value.

### `fast_node_removal_rate`

The amount of nodes to taint whenever the node group utilisation goes below the 
`taint_lower_capacity_threshold_percent` value.

### `scale_up_threshold_percent`

This value defines the threshold at which Escalator will increase the size of the node group. Escalator will
calculate based on the current utilisation how much to increase the node group to be below the value of this
option. For example:

`scale_up_threshold_percent` is configured as **70** and the current node group utilisation is **75%**, Escalator will
increase the size of the node group so that the utilisation drops below **70**.

Escalator calculates the utilisation based on the CPU and Memory limits/requests of containers running in the node
group and uses whichever is the higher value as the utilisation. More information on utilisation calculation can be
found [here](../calculations.md).

[**Slack space**](./advanced-configuration.md) can be configured by leaving a gap between the 
`scale_up_threshold_percent` and `100%`, e.g. a value of `70` will mean `30%` slack space.

### `scale_up_cool_down_period` and `scale_up_cool_down_timeout`

`scale_up_cool_down_period` is a grace period before Escalator can consider the scale up of the node group
a success. This grace period is managed by the scale up locking mechanism and will not unlock the lock mechanism until
at least the `scale_up_cool_down_period` has passed.

A successful scale up is when the ASG size matches the ASG target size and the ready nodes in Kubernetes matches the
ASG target size.

`scale_up_cool_down_timeout` is a timeout that when reached Escalator will release the scale lock so it can do future
scale up activities. This timeout is often reached when the cloud provider couldn't bring the nodes up in time or when
the node failed to register with Kubernetes in time.

Having the scale up activity timeout isn't necessarily a bad thing, it just acts as a fail safe in case scaling 
activities take too long so that the scale lock isn't permanently enabled.

### `soft_delete_grace_period` and `hard_delete_grace_period`

These values define the periods before a node is attempted to be terminated and when the node is forcefully terminated.

When a node is tainted, the time at which it was tainted is recorded with the taint. `soft_delete_grace_period` defines
the time until Escalator can try to terminate the node. During the time period after `soft_delete_grace_period` and
before `hard_delete_grace_period`, Escalator will only terminate the node if it is considered empty. A node is
considered empty if it doesn't have any sacred pods running on it. Daemonsets are filtered out of this check.

If `hard_delete_grace_period` is reached, the node will be terminated regardless, even if there are sacred pods 
running on it.

Take consideration when setting `soft_delete_grace_period`, as a low value will mean the node is terminated as soon as
possible, but if there is a sudden spike in pods there may not be an available pool of tainted nodes to untaint.

It is highly recommended to have the `hard_delete_grace_period` option set to a large value to give pods running on 
nodes enough time to finish before the node is terminated. 

Logic for determining if a node is empty can be found in `pkg/k8s` `NodeEmpty()`
