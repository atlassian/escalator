# Escalator
Batch Optimized Horizontal Autoscaler for Kubernetes

Escalator is a cluster-level autoscaler for Kubernetes that is designed for batch workloads that cannot be fast-drained. Kubernetes is a container orchestration framework that schedules Docker containers on a cluster.

The key preliminary features are:

- Cluster-level utilisation node scaling.
- Safely drain "sacred" pods from nodes to allow graceful termination
- Designed to work on selected auto-scaling groups, to allow the default Kubernetes autoscaler to continue to scale our service based workloads
- Automatically cycle oldest nodes

The need for this autoscaler is dervied from our own experiences with very large batch workloads being scheduled that can't be force-drained by the default autoscaler.

## Documentation and Design
See [Docs](docs/)