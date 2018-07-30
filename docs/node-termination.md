# Node Termination

## Node selection method for termination

At the time of writing, Escalator has only a single method for determining which nodes to terminate first when scaling
down, this method is called "oldest first". In the future, different node selection methods will be available.  

### Oldest first

By default, Escalator will terminate the oldest nodes first when it is scaling down. This is achieved by looking at when
the node joined the Kubernetes API (creation time), and prioritising nodes that were created earliest.

The bulk of the logic for determining the oldest nodes is done in the [sort.go](../pkg/controller/sort.go) file, 
where we compare the creation timestamps of each node.

This method is useful to ensure there are always new nodes in the cluster. If you want to deploy a configuration change
to your nodes, you can use Escalator to cycle the nodes by terminating the oldest first until all of the nodes are
using the latest configuration.