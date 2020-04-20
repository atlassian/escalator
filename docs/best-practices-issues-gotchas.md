# Best Practices, Common Issues and Gotchas

## Best Practices

 - Run Escalator with `--drymode` enabled when initially deploying it to a production environment to get an idea of
   how it will react before letting it taint/untaint and terminate instances.

 - Initially run Escalator with safe or moderate threshold values when deploying it to production for the first time,
   tune the values accordingly once Escalator is running as desired in your environment.

 - Run it with a high `hard_delete_grace_period` timeout to prevent Escalator terminating nodes that are
   still running workloads.

 - Run Escalator with a low scan interval, for example 30 or 60 seconds. This will ensure Escalator is responsive
   enough during a spike in load or pods.

 - Run Escalator with leader election enabled, in a HA deployment (that is, >1 replica). Turn this behaviour on with
   `--leader-elect`, see the [Command line options](./configuration/command-line.md) docs for more info. You can also
   inspect the leader events with `kubectl describe lease <lease-name>`, where the default Lease name is
   `escalator-leader-elect`.

## Common Issues & Gotchas

 - Ensure scale in protection is enabled for the cloud provider node group. This will prevent the cloud provider from
   terminating instances in the rare case where there are more instances than the desired node group size.

 - It is recommended to match Escalator `min_nodes` and `max_nodes` to the value in the cloud provider. This will 
   prevent weird cases where Escalator will try to scale down but will be blocked by the cloud provider.

 - Escalator only supports one cloud provider per deployment. You will need to run multiple different deployments of 
   Escalator inside the cluster to use more than one cloud provider.

 - Escalator only supports one cloud provider region per deploy. As with running multiple cloud providers, multiple
   different deployments of Escalator will need to be used.

 - Escalator only supports running the same type of instance (e.g. instances with different CPU and memory
   configurations) in a node group in the cloud provider. If you would like to use different types of instances and
   still have have Escalator manage them, you will need to place each instance type in it's own node group in the
   cloud provider. Pods

 - Escalator does not support scaling down to zero nodes. This is because we require at least one node to calculate the
   current utilisation.
