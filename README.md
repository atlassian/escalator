# Escalator

[![Godoc](https://godoc.org/github.com/atlassian/escalator?status.svg)](https://godoc.org/github.com/atlassian/escalator)
[![Build Status](https://travis-ci.org/atlassian/escalator.svg?branch=master)](https://travis-ci.org/atlassian/escalator)
[![Go Report Card](https://goreportcard.com/badge/github.com/atlassian/escalator)](https://goreportcard.com/report/github.com/atlassian/escalator)
[![license](https://img.shields.io/github/license/atlassian/escalator.svg)](LICENSE)

**Escalator is a batch or job optimized horizontal autoscaler for Kubernetes**

It is designed for large batch or job based workloads that cannot be force-drained and moved when the cluster needs to 
scale down - Escalator will ensure pods have been completed on nodes before terminating them. It is also optimised for 
scaling up the cluster as fast as possible to ensure pods are not left in a pending state.

## Key Features

- Calculate requests and capacity to determine whether to scale up, down or to stay at the current scale
- Waits until non-daemonset pods on nodes have completed before terminating the node
- Designed to work on selected auto-scaling groups to allow the default
  [Kubernetes Autoscaler](https://github.com/kubernetes/autoscaler) to continue to scale service based workloads
- Automatically terminate oldest nodes first
- Support for slack space to ensure extra space in the event of a spike of scheduled pods
- Does not terminate or factor cordoned nodes into calculations - allows cordoned nodes to persist for debugging 
- Support for different cloud providers - AWS only at the moment
- Scaling and utilisation metrics
- Leader election so you can run a HA Deployment inside a cluster.
- Basic support for multiple different types of instances in a Node Group.

The need for this autoscaler is derived from our own experiences with very large batch workloads being scheduled and the
default autoscaler not scaling up the cluster fast enough. These workloads can't be force-drained by the default 
autoscaler and must complete before the node can be terminated.

## Planned Features & Development Roadmap

- [#93](https://github.com/atlassian/escalator/issues/93) - Add option to perform a drain before terminating a node
- [#60](https://github.com/atlassian/escalator/issues/60) - Add additional metrics
- [#71](https://github.com/atlassian/escalator/issues/71) - Generate unique ID for each scale activity

## Documentation and Design

See [Docs](docs/README.md)

## Requirements

- [Kubernetes](https://kubernetes.io/) version 1.8+. Escalator has been tested and deployed on 1.8+ and newer. Older 
versions of Kubernetes may have bugs or issues that will prevent it from functioning properly.
- [Go](https://golang.org/) version 1.12+ with `GO111MODULE=on`, but Go 1.13+ is highly recommended as Escalator uses go modules for dependencies.
- Dependencies and their locked versions can be found in `go.mod` and `go.sum`.

## Building

```bash
# If using Go 1.12 make sure to enable Go modules support. For Go 1.13 you can ignore this
export GO111MODULE=on
```

```bash
# Fetch dependencies and build Escalator
make build
```

## How to run - Quick Start

### Locally (out of cluster)

```bash
go run cmd/main.go --kubeconfig=~/.kube/config --nodegroups=nodegroups_config.yaml
```

### Deployment (in cluster)

See [Deployment](./docs/deployment/README.md) for full Deployment documentation.

```bash
# Build the docker image
docker build -t atlassian/escalator .

# Create RBAC configuration
kubectl create -f docs/deployment/escalator-rbac.yaml

# Create config map - modify to suit your needs
kubectl create -f docs/deployment/escalator-cm.yaml

# Create deployment
kubectl create -f docs/deployment/escalator-deployment.yaml
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
* Follow the existing style (we are using [goreturns](https://github.com/sqs/goreturns) to format and lint escalator)
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

* [CLA for corporate contributors](https://opensource.atlassian.com/corporate)
* [CLA for individuals](https://opensource.atlassian.com/individual)

## License

Copyright (c) 2018 Atlassian and others.
Apache 2.0 licensed, see [LICENSE](./LICENSE) file.
