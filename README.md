# Escalator

Batch Optimized Horizontal Autoscaler for Kubernetes.

Escalator is a cluster-level autoscaler for Kubernetes that is designed for very large batch workloads that cannot be 
fast-drained, but also where the default autoscaler does not scale up fast enough. 
Kubernetes is a container orchestration framework that schedules Docker containers on a cluster.

**The key preliminary features are:**

- Cluster-level utilisation node scaling.
- Calculate requests and capacity to determine whether to scale up, down or to stay at the current scale
- Wait until non-daemonset pods on nodes have completed before terminating the node
- Designed to work on selected auto-scaling groups to allow the default
  [Kubernetes Autoscaler](https://github.com/kubernetes/autoscaler) to continue to scale our service based workloads
- Automatically terminate oldest nodes first
- Support for different cloud providers - only AWS at the moment

The need for this autoscaler is derived from our own experiences with very large batch workloads being scheduled and the
default autoscaler not scaling up the cluster fast enough. These workloads can't be force-drained by the default autoscaler
and must complete before the node can be terminated.

## Documentation and Design

See [Docs](docs/README.md)

## Building

```bash
make setup
make build
```

## How to run

### Locally (out of cluster)

```bash
go run cmd/main.go --kubeconfig=~/.kube/config --nodegroups=nodegroups.yaml
```

### Inside cluster

```bash
make docker-build
```

In the Escalator Kubernetes deployment:

```yaml
# You need to mount your config file into your container
- image: atlassian/escalator
  command:
  - ./main
  - --nodegroups
  - /opt/conf/nodegroups/nodegroups_config.yaml
```

#### nodegroups_config.yaml example

```yaml
node_groups:
  - name: "shared"
    label_key: "customer"
    label_value: "shared"
    cloud_provider_asg: "shared-nodes"
    min_nodes: 1
    max_nodes: 30
    dry_mode: false
    taint_upper_capacity_threshhold_percent: 40
    taint_lower_capacity_threshhold_percent: 10
    slow_node_removal_rate: 2
    fast_node_removal_rate: 5
    scale_up_threshhold_percent: 70
    scale_up_cool_down_period: 2m
    scale_up_cool_down_timeout: 10m
    soft_delete_grace_period: 1m
    hard_delete_grace_period: 10m

```

## Configuring

See [Configuration](docs/configuration/README.md)

## Testing

#### Test everything

```bash
make test
```

#### Test a specific package

To test the controller package:
```bash
go test ./pkg/controller
```
