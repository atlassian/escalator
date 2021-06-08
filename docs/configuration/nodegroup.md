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
    taint_effect: NoExecute
    aws:
        fleet_instance_ready_timeout: 1m
        launch_template_id: lt-1a2b3c4d
        launch_template_version: "1"
        lifecycle: on-demand
        instance_type_overrides: ["t2.large", "t3.large"]
        resource_tagging: false
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

#### Auto Discovery

`min_nodes` and `max_nodes` can be auto-discovered by Escalator and set to the min and max node values that are
configured for the node group in the cloud provider. These values are populated only when Escalator starts for the first
time. If the values in the cloud provider have changed, you will have to stop and restart Escalator for it to pick up
the new values.

To enable this, set `min_nodes` and `max_nodes` to `0` for the node group in `nodegroups_config.yaml` or simply remove
the two options from `nodegroups_config.yaml`.

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

If the node group utilisation is **5%**, and `taint_lower_capacity_threshold_percent` is configured as **10**,
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

### `taint_effect`

This is an optional field and the value defines the taint effect that will be applied to the nodes when scaling down.
The valid values are :

- NoSchedule
- NoExecute
- PreferNoSchedule

IF not set, it will default to NoSchedule.

### `aws.fleet_instance_ready_timeout`

This is an optional field. The default value is 1 minute.

When using the one-shot capacity acquisition for AWS (see `aws.launch_template_id`), this is the maximum amount of time
that Escalator will block waiting for new EC2 instances to become ready so that they can be added to the node group.
This roughly corresponds to the amount of time it takes an instance to boot to multi-user mode and for the EC2 control
plane to notice that it is healthy. This generally takes much less than a minute.

**Note:** Escalator will block other scaling operations for, at most, this amount of time while new capacity comes
online.

### `aws.launch_template_id`

This value is the launch template ID (of the format `lt-[a-f0-9]{17}`) to use as a template for new instances that are
acquired using the AWS Fleet API. This value can be obtained through the AWS EC2 console on the Launch Templates page
or from the `LaunchTemplateId` field returned from the
[create-launch-template](https://docs.aws.amazon.com/cli/latest/reference/ec2/create-launch-template.html) CLI command
and AWS API call.

Setting this value and that of `aws.launch_template_version` allows Escalator to use the AWS Fleet API to acquire all
desired capacity for a scale-up operation at once rather than waiting for an auto-scaling group to add capacity. This
call may fail if AWS is unable to fulfill all capacity for some reason. Escalator will wait for the acquired capacity
to become ready on the AWS side and will attach it all to the Escalator managed auto-scaling-group.

The instance types configured in the launch template should match the instance types configured for the auto-scaling
group. If the auto-scaling group was created using a launch template instead of a launch configuration then that
template ID should be used here.

**Note:** the AWS Fleet API does not support balancing requested instances by availability zone. When using this
functionality the desired capacity will be acquire all at once in a single availability zone or not at all. This may
change in a future update to the API.

### `aws.launch_template_version`

This value is the version of the launch template to use. See `aws.launch_template_id` above. This value should be a
numeric string. This value can be obtained through the AWS EC2 console on the Launch Templates page or from the
`LatestVersionNumber` or `DefaultVersionNumber` field returned from the
[create-launch-template](https://docs.aws.amazon.com/cli/latest/reference/ec2/create-launch-template.html) CLI command
and AWS API call.

### `aws.lifecyle`

Dependent on Launch Template ID being specified.

This optional value is the lifecycle of the instances that will be launched. The accepted values are strings of either
`on-demand` or `spot` to request On-Demand or Spot instances respectively. If no value is specified this will default
to `on-demand`.

### `aws.instance_type_overrides`

Dependent on Launch Template ID being specified.

An optional list of instance types to override the instance type within the launch template. Providing multiple instance
types here increases the likelihood of a Spot request being successful. If omitted the instance type to request will
be taken from the launch template.

### `aws.resource_tagging`

Tag ASG and Fleet Request resources used by Escalator with the metatdata key-value pair
`k8s.io/atlassian-escalator/enabled`:`true`. Tagging doesn't alter the functionality of Escalator. Read more about
tagging your AWS resources [here](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/Using_Tags.html).
