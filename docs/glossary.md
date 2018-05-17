# Glossary

## node group

A group of nodes that reside in the same scaling group within a cloud provider, but are also labelled with the same
label, e.g. customer: shared

## cloud provider

A service that provides compute resources, e.g. AWS, GCP, Azure.

## auto scaling group or ASG

AWS service that groups EC2 instances so that they can be scaled and managed together. Read more about them 
[here](https://docs.aws.amazon.com/autoscaling/ec2/userguide/AutoScalingGroup.html).

## tainting

When we refer to "tainting" a node, we refer to applying a Kubernetes taint to the node that prevents scheduling of pods
to that node. We apply the "NoSchedule" effect when we taint a node. More information on tainting of nodes can be found 
[here](https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/).

## untainting

To remove the "NoSchedule" effect from a node. See tainting above.

## slack space

Extra nodes on top of the needed capacity to run the pods for a node group. This ensures there is capacity for 
daemonsets to run on each node, as well as some extra capacity to handle a spike in requested resources. 
Read about slack space [here](./configuration/advanced-configuration.md).

## utilisation

The overall node group resource use, based on the resource requests from pods and the allocatable resources from nodes.
