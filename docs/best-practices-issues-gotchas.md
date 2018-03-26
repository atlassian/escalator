# Best Practices, Common Issues and Gotchas

## Best Practices

 - Run Escalator with `--drymode` enabled when initially deploying it to a production environment to get an idea of
   how it will react before letting it taint/untaint and terminate instances.
 - Initially run Escalator with safe or moderate threshold values when deploying it to production for the first time,
   tune the values accordingly once Escalator is running as desired in your environment.
 - Run it with a high `hard_delete_grace_period` timeout to prevent Escalator terminating nodes that are
   still running workloads.

## Common Issues & Gotchas

 - Ensure scale in protection is enabled for the cloud provider node group. This will prevent the cloud provider from
   terminating instances in the rare case where there are more instances than the desired node group size.
