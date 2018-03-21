# Escalator

**Escalator is a batch or job optimized horizontal autoscaler for Kubernetes**

It is designed for large batch or job based workloads that cannot be force-drained and moved when the cluster needs to 
scale up - Escalator will ensure pods have been completed on nodes before terminating them. It is also optimised for 
scaling down the cluster as fast as possible to ensure pods are not left in a pending state.

## Key Features

- Utilisation based node scaling
- Calculate requests and capacity to determine whether to scale up, down or to stay at the current scale
- Waits until non-daemonset pods on nodes have completed before terminating the node
- Designed to work on selected auto-scaling groups to allow the default
  [Kubernetes Autoscaler](https://github.com/kubernetes/autoscaler) to continue to scale service based workloads
- Automatically terminate oldest nodes first
- Support for slack space to ensure extra space in the event of a spike of scheduled pods
- Does not terminate or factor cordoned nodes into calculations - allows cordoned nodes to persist for debugging 
- Support for different cloud providers - AWS only at the moment
- Scaling and utilisation metrics

The need for this autoscaler is derived from our own experiences with very large batch workloads being scheduled and the
default autoscaler not scaling up the cluster fast enough. These workloads can't be force-drained by the default 
autoscaler and must complete before the node can be terminated.

## Development Roadmap

- [#56](https://github.com/atlassian/escalator/issues/56) - Implement leader election mechanism
- [#57](https://github.com/atlassian/escalator/issues/57) - Implement healthcheck endpoint
- [#60](https://github.com/atlassian/escalator/issues/60) - Add additional metrics
- [#71](https://github.com/atlassian/escalator/issues/71) - Generate unique ID for each scale activity

## Documentation and Design

See [Docs](docs/README.md)

## Requirements

- [Kubernetes](https://kubernetes.io/) version 1.8+. Escalator has been tested and deployed on 1.8+ and newer and older 
versions of Kubernetes may have bugs or issues that will prevent it from functioning properly.
- [Dep](https://golang.github.io/dep/). It is recommended to use a recent release from 
[https://github.com/golang/dep/releases](https://github.com/golang/dep/releases)
- [Go](https://golang.org/) version 1.8+, but newer versions of Go are highly recommended.
- Dependencies and their locked versions can be found in `Gopkg.toml` and `Gopkg.lock`.

## Building

```bash
// Install dependencies
make setup
// Build Escalator
make build
```

## How to run

### Locally (out of cluster)

```bash
go run cmd/main.go --kubeconfig=~/.kube/config --nodegroups=nodegroups_config.yaml
```

### Deployment (in cluster)

See [Deployment](./docs/deployment/README.md) for full Deployment documentation.

```bash
// Build the docker image
docker build -t atlassian/escalator .
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

```bash
make test
```

### Test a specific package

For example, to test the controller package:

```bash
go test ./pkg/controller
```

## Contributors

Pull requests, issues and comments welcome. For pull requests:

* Add tests for new features and bug fixes
* Follow the existing style
* Separate unrelated changes into multiple pull requests

See the existing issues for things to start contributing.

For bigger changes, make sure you start a discussion first by creating
an issue and explaining the intended change.

Atlassian requires contributors to sign a Contributor License Agreement,
known as a CLA. This serves as a record stating that the contributor is
entitled to contribute the code/documentation/translation to the project
and is willing to have it used in distributions and derivative works
(or is willing to transfer ownership).

Prior to accepting your contributions we ask that you please follow the appropriate
link below to digitally sign the CLA. The Corporate CLA is for those who are
contributing as a member of an organization and the individual CLA is for
those contributing as an individual.

* [CLA for corporate contributors](https://na2.docusign.net/Member/PowerFormSigning.aspx?PowerFormId=e1c17c66-ca4d-4aab-a953-2c231af4a20b)
* [CLA for individuals](https://na2.docusign.net/Member/PowerFormSigning.aspx?PowerFormId=3f94fbdc-2fbe-46ac-b14c-5d152700ae5d)

## License

Copyright (c) 2018 Atlassian and others.
Apache 2.0 licensed, see [LICENSE](./LICENSE) file.
