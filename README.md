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

## How to run

### Locally
```
go run cmd.main.go--kubeconfig ~/.kube/config --nodegroups nodegroups.yaml
```

### Docker
```
# will build and push the docker image to the registry
make push
```
In the escalator-deployment:
```
# need to mount your config file to your container
- image: docker.atl-paas.net/kitt/escalator:{{ escalator.version }}
  command:
  - ./main
  - --nodegroups
  - /opt/conf/nodegroups/nodegroups_config.yaml
```

## How to test
#### Test everything
```
go test ./...
```
#### Test a package
```
go test ./pkg/<package-name> 
```

### NodeGroupConfig minimum yaml example
```
node_groups:
  - name: "buildeng"
    label_key: "customer"
    label_value: "buildeng"
    min_nodes: 5
    max_nodes: 300
    dry_mode: true
    taint_upper_capacity_threshhold_percent: 70
    taint_lower_capacity_threshhold_percent: 50
    untaint_upper_capacity_threshhold_percent: 95
    untaint_lower_capacity_threshhold_percent: 90
    slow_node_removal_rate: 2
    fast_node_removal_rate: 3
    slow_node_revival_rate: 2
    fast_node_revival_rate: 3
  - name: "default"
    label_key: "customer"
    label_value: "shared"
    min_nodes: 1
    max_nodes: 10
    dry_mode: true
    taint_upper_capacity_threshhold_percent: 25
    taint_lower_capacity_threshhold_percent: 20
    untaint_upper_capacity_threshhold_percent: 45
    untaint_lower_capacity_threshhold_percent: 30
    slow_node_removal_rate: 2
    fast_node_removal_rate: 3
    slow_node_revival_rate: 2
    fast_node_revival_rate: 3
```