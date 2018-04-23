# Glossary

## node group

A group of nodes that reside in the same scaling group within a cloud provider, but are also labelled with the same
label, e.g. customer: shared

## cloud provider

A service that provides compute resources, e.g. AWS, GCP.

## auto scaling group or ASG

AWS service that groups EC2 instances so that they can be scaled and managed together. Read more about them 
[here](https://docs.aws.amazon.com/autoscaling/ec2/userguide/AutoScalingGroup.html).

## tainting or "to taint a node"

When we refer to "tainting" a node, we refer to apply a Kubernetes taint to the node that prevents scheduling of pods
to that node. We apply the "NoSchedule" effect when we taint a node. More information on tainting of nodes can be found 
[here](https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/).

Tainting a node differs slightly to the `cordon` command in `kubectl`. The cordon command toggles the 
`node.Spec.Unschedulable` field, whereas when Escalator taints a node to be unschedulable, we add a "NoSchedule" taint.

## untainting or "to untaint a node"

To remove the "NoSchedule" effect from a node. See tainting above.

## slack space

Extra nodes on top of the needed capacity to run the pods for a node group. This ensures there is capacity for 
daemonsets to run on each node, as well as some extra capacity to handle a spike in requested resources. 
Read about slack space [here](./configuration/advanced-configuration.md).

## utilisation

The overall node group resource use, based on the resource requests from pods and the allocatable resources from nodes.

## draining or "to drain a node"

Draining a node involves evicting or deleting all non-daemonset pods on the node so that the node can be deleted
without any sacred pods running on it.

## sacred pod

A pod that is considered "sacred" is one that is not a daemonset

## desired size


