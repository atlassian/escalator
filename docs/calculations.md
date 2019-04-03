# Algorithm and Calculations

## Assumptions

To perform the requests, capacity, utilisation and scale up delta calculations, Escalator makes the following
important assumptions:

 - **All nodes in the node group have the same allocatable resources**
 - **All containers in pods in the node group have specified resource requests**
 
**If Escalator is unable to make the above assumptions, e.g. there are containers without resource requests specified or
nodes have different allocatable resources, scaling activities (scaling up or down) may have unintended side affects.**

## Requests, capacity and utilisation

To work out how much to scale up the node group by, Escalator performs a calculation based on the current requests in 
the node group and compares it against the current capacity of the node group. 

To achieve this, all of the containers in the pods of the node group have their requests are added together. 
The allocatable resources (capacity) of all of the nodes are also added together. 

The requests are then compared against the capacity of the nodes and a percentage utilisation is generated for both CPU and
memory. Escalator then takes the higher of the two (CPU and Memory) and uses it for any subsequent calculations.

**For example:**

We have 10 pods with a single container, each container requesting `500m` CPU and `100mb` memory.
The calculated requests would add up to 5 (`5000m`) CPU and `1000mb` memory.
 - `10 * 500m` = **5000m**
 - `10 * 100mb` = **1000mb** 

We have 2 nodes with allocatable resources of 1 (`1000m`) CPU and `4000mb` memory each.
The calculated capacity would add up to 2 (`2000m`) CPU and `8000mb` memory.
 - `2 * 1000m` = **2000m**
 - `2 * 4000mb` = **8000mb**

The utilisation would then be calculated as follows:
 - CPU: `5000m / 2000m * 100` = **250%**
 - Memory: `1000mb / 8000mb * 100` = **12.5%**
 
We then take the higher percentage utilisation, in this case CPU: **250%**.

When node group's minimum size is set to 0, a special value (math.MaxFloat64) is used for CPU/Memory percentage utilisation if
it scales up from 0. This value will be used to calculate scale up delta as describe below.

Based on this figure we will then either scale up, do nothing or scale down. This depends on what the thresholds are 
configured at. Threshold configuration is [documented here](./configuration/advanced-configuration.md).

## Scale up delta

When it is determined that Escalator needs to scale up the node group, it needs to perform a calculation to determine
how much to tell the cloud provider to scale up the node group by.

The delta is calculated through the use of a percent decrease formula. We need to calculate how much to increase the
node group by calculating the amount of nodes needed to decrease the utilisation to be below the 
`scale_up_threshold_percent` option.

**For example:**

- `scale_up_threshold_percent` is `70`
- CPU utilisation is `250`
- `(250 - 70) / 70` = `2.57142857143`
- `2.57142857143 * 2 nodes` = `5.14285714286`
- Amount to increase by: `ceil(5.14285714286)` = `6` nodes

By requesting the node group to scale up by `6` nodes, the new total node count will be `8`. With a new node count of 
`8`, the node group utilisation is as follows:

The utilisation would then be calculated as follows:
 - CPU: `5000m / 8000m * 100` = **62.5%**
 - Memory: `1000mb / 32000mb * 100` = **3.125%**

 If the utilisation is the special value (math.MaxFloat64) mentioned above which means it scales up from 0, escalator will look up node group state for cached version of node allocatable capacity and then calculate the delta based on that. Otherwise it will just increase the node group by 1 when cached value doesn't exist, for example escalator tries to scale up from 0 right after it starts.

 **For example:**

when cached capacity exists:
- `scale_up_threshold_percent` is `70`
- cached node CPU allocatable capacity is `1000m`
- pods CPU request is `1800m`
- `1800m/1000m/70*100` = `2.57100`
- Amount to increase by: `ceil(2.57100)` = `3` nodes

when cached capacity doesn't exist:
- Amount to increase by: `1` node


## Daemonsets

[Daemonsets](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/) are copies of pods that run on all 
nodes in a cluster. Daemonsets are purposely factored out of the utilisation calculations that Escalator performs.
This is done for a variety of reasons, these being:

 - Keeping the calculations for utilisation simple by only looking at the pods that have the appropriate node selector
   or node affinity set.
 - There is no simple way to select the daemonsets that are running in the node group, as they don't have any 
   node selectors or node affinity.
 - There is no simple way to calculate the utilisation of the daemonsets on any new nodes that are to be brought up.
 
 **To mitigate this caveat, it is highly recommended that slack space is configured for the node group to cater for 
 daemonsets. [More information on slack space](./configuration/advanced-configuration.md).**
