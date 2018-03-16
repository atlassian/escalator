# Algorithm and Calculations

## Assumptions

To perform the requests, capacity, utilisation and scale up delta calculations, Escalator makes the following
important assumptions:

 - **All nodes in the node group have the same allocatable resources**
 - **All containers in the node group have specified resource requests**

## Requests, capacity and utilisation

To work out how much to scale up the node group by, Escalator performs a calculation based on the current requests in 
the node group and compares it against the current capacity of the node group. 

To achieve this, all of the containers in the pods of the node group have their requests are added together. 
The allocatable resources (capacity) of all of the nodes are also added together. 

The requests are then compared against the capacity of the nodes and a percentage utilisation is generated for both CPU and
memory. Escalator then takes the higher of the two (CPU and Memory) and uses it for any subsequent calculations.

**For example:**

We have 10 pods with a single container, each requesting `500m` CPU and `100mb` memory.
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

Based on this figure we will then either scale up, do nothing or scale down. This depends on what the thresholds are 
configured at. Threshold configuration is [documented here](./configuration/advanced-configuration.md).

## Scale up delta

When it is determined that Escalator needs to scale up the node group, it needs to perform a calculation to determine
how much to tell the cloud provider to scale up the node group by.

The delta is calculated by first figuring out the worth of each node running in the node group. The node worth is 
a percentage a single makes up of the total node group.

**For example:**

- **100 nodes**, the node worth will be **1.0**, i.e. 1 node is **1% of the total capacity**
- **10 nodes**, the node worth will be **10.0**, i.e. 1 node is **10% of the total capacity**
- **37 nodes**, the node worth will be **2.7027027**, i.e. 1 node is **2.7027027% of the total capacity**

To then calculate the scale up delta, we get the remaining percentage needed to bring the overall utilisation below the
scale up threshold. With this remaining percentage and the node worth we can calculate the node delta.

**For example:**

- CPU Utilisation = `250%`
- Scale up threshold (`scale_up_threshhold_percent`) = `70`
- Node worth (2 nodes) = `50.0`
- Remaining percentage needed to be below scale up threshold: `250 - 70` = `180`
- Scale up delta: `180 / 50` = `3.6`
- Amount sent to cloud provider to increase by: `ceil(3.6)` = `4` 
- We `ceil()` the final scale up delta because we can only request whole nodes

By requesting the node group to scale up by `4`, the new total node count will equal `6`. With a new node count of `6`,
the node group utilisation is as follows:

The utilisation would then be calculated as follows:
 - CPU: `5000m / 6000m * 100` = **83.3334%**
 - Memory: `1000mb / 24000mb * 100` = **0.04167%**

## Daemonsets

