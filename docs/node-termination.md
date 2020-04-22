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

### Annotating nodes / Stopping termination of selected nodes

For most cases when wanting to exclude a node from termination for maintenance, you should first consider the [**cordon function**](./scale-process).
Cordoning a node filters it out from calculation **and** scaling processes (tainting and deletion). Additionally, cordoning a node in Kubernetes marks it Unschedule-able, so it won't accept workloads while in this state.
 
In the cases where Cordoning is not acceptable, because you want your node to continue to receive workloads but not be deleted, you can annotate the node to tell escalator to treat it as normal, except for stopping it from being deleted.
This means the node will still factor into scaling calculations, and get tainted, untainted as the cluster scales, but if it is in a situation where it would be deleted normally, escalator will skip it and leave it active.

#### To annotate a node:

**Annotation key:** `atlassian.com/no-delete`

The annotation value can be any **non empty** string that conforms to the Kubernetes annotation value standards. This can usually indicate the reason for annotation which will appear in logs

**Example:**

`kubectl edit node <node-name>`

**Simple example:**
```yaml
apiVersion: v1
kind: Node
metadata:
  annotations:
    atlassian.com/no-delete: "true"
```

**An elaborate reason:**
```yaml
apiVersion: v1
kind: Node
metadata:
  annotations:
    atlassian.com/no-delete: "testing some long running bpf tracing"
```
