# Scale Process

The following is documentation on the process that Escalator follows for scaling up and scaling down the node group.

## Scale up

1. Get all of the pods in the node group
1. Get all of the nodes in the node group
1. Filter the nodes into three categories:
    1. Untainted, tainted and cordoned
1. Calculate the requests from the pods
1. Calculate the allocatable capacity from the untainted nodes
1. Calculate the percentage utilisation using the requests and capacity
1. If the scale lock is present, ensure the scale lock has been released before proceeding
    1. Full details on the scale lock can be found below
1. Determine which is greater, the CPU or the Memory utilisation
1. Determine whether we need to scale up, scale down or do nothing
1. In this case we need to scale up, calculate the amount of nodes we need to increase by
    1. Scale up calculations can be found [here](./calculations.md)
1. Scale up the node group by the amount of nodes needed
    1. Attempt to untaint nodes first
    1. If we still need more nodes, issue a request to the cloud provider to increase the node group
    1. If we requested the cloud provider to scale up, lock the scale lock

## Scale down

1. Get all of the pods in the node group
1. Get all of the nodes in the node group
1. Filter the nodes into three categories:
    1. Untainted, tainted and cordoned
1. Calculate the requests from the pods
1. Calculate the allocatable capacity from the untainted nodes
1. Calculate the percentage utilisation using the requests and capacity
1. If the scale lock is present, ensure the scale lock has been released before proceeding
    1. Full details on the scale lock can be found below
1. Determine which is greater, the CPU or the Memory utilisation
1. Determine whether we need to scale up, scale down or do nothing
1. In this case we need to scale down
1. Determine whether we need to perform a "fast" scale down or "slow" scale down
    1. Fast and slow node removal is configured per node group, documentation [here](./configuration/nodegroup.md)
1. Scale down the node group by the amount of nodes needed
    1. Select nodes for termination - see [Node Termination](./node-termination.md) for the method we use for selecting
       which nodes to terminate
    1. Remove any nodes that have already been tainted and have exceed the grace period and are considered empty
        1. Tell the cloud provider to delete the node from the node group
        1. Delete the node from Kubernetes
    1. Taint nodes, based on the "fast" or "slow" scale down amounts
         

## Scale lock

The scale lock is a mechanism to ensure that the requested scale up amount to the cloud provider is successful before
requesting additional scale ups or scaling down.

This helps prevent cases where there may be an "infinite" scale up due to the delay in the time it takes for nodes to 
appear in Kubernetes after they have been created by the cloud provider.

It also prevents Escalator from scaling down whilst the cloud provider is mid way through bringing new nodes up. It
allows the scale up activity to safely finish before performing any additional actions that will impact the node
group.

The scale lock is checked before any scaling activity is done - scale up, scale down or doing nothing. 

The scale lock is configured using two options - `scale_up_cool_down_period` and `scale_up_cool_down_timeout`. These
control the minimum time that the scale lock has to be locked before unlocking it, and the maximum time the scale lock
can be locked for. After the timeout has been reached, the lock is forcefully unlocked.

## Tainting of nodes

Tainting of nodes involves applying a "NoSchedule" effect to the node. When applying the "NoSchedule" taint to the node,
we use the current timestamp of when the taint was applied so we can apply grace periods to deleting the node.

Escalator taints are given the `atlassian.com/escalator` key. 

## Cordoning of nodes

Escalator does not use the cordoning command anywhere in it's process. This is done to preserve the cordoning command
for system administrators to filter the cordoned node out of calculations. This way, a faulty or misbehaving node
can be cordoned by the system administrator to be debugged or troubleshooted without worrying about the node being 
tainted and then terminated by Escalator.

