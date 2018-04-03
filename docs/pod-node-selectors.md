# Pod and Node Selectors

For pods and nodes to be considered in the calculations for a node group, they must be appropriately labelled.
Escalator uses the configured label and label value in the `nodegroups_config.yaml` file for this.

## Nodes

Nodes are selected if they have a label matching the `label_key` and value matching the `label_value` in the 
`nodegroups_config.yaml` file.

You can see which nodes Escalator will monitor with the following `kubectl` command:

`kubectl get nodes --selector=customer=shared`

You can see the function that performs the filtering of nodes in the 
[`pkg/controller/node_group.go` file](../pkg/controller/node_group.go), specifically `NewNodeLabelFilterFunc()`.

## Pods

Selecting pods is a little more complex, as it considers the pod's nodeSelector, as well as the pod's nodeAffinity.

The following pod node selector will include the pod in the `customer=shared` node group:

```yaml
spec:
  nodeSelector:
    customer: shared
```

The following pod node affinity will include the pod in the `customer=shared` node group:

```yaml
spec:
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: customer
            operator: In
            values:
            - shared
```

Replace `customer` and `shared` with your node/pod label key-value.

You can see the function that performs the filtering of pods in the 
[`pkg/controller/node_group.go` file](../pkg/controller/node_group.go), specifically `NewPodAffinityFilterFunc()`.

## Default Pod Selector

Escalator includes an option to include pods into the utilisation calculations for pods that don't have a `nodeSelector`
or a `nodeAffinity` specified. This is useful when running a "shared" node group that picks up any pods that don't run
on a specific node. 

When this option is used, a different pod filtering method is utilised:

```go
// allow pods without a node selector and without a pod affinity
return len(pod.Spec.NodeSelector) == 0 && pod.Spec.Affinity == nil
```

To use this option, name the node group `default` in the `nodegroups_config.yaml` file like so:

```yaml
node_groups:
  - name: "default"
    label_key: "customer"
    label_value: "shared"
``` 

`label_key` and `label_value` is still used for selecting which nodes are included in the capacity calculations.

## More information

- More information on node labels, node selectors and node affinity can be found 
  [here](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/).
- More information on Labels and Selectors can be found 
  [here](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/).
