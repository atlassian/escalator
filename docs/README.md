# Documentation
General system documentation

# How to use the stash remote
The SOX remote stash repo named stash is used to pin a specific version of escalator down in compliance with peer review and CI/CD standards
To remote a branch to master of stash:
- PR branch on github and merge
- Create branch username/stash-release-description
- `git push -u stash username/stash-release-description`
- Create PR on stash and merge

# Dependencies
We used `godep` for dependency managment
## Install: 
- `go get github.com/tools/godep`
## To add a package: 
1. `go get your.package/name`
2. add `import "your.package/name"` to your code
3. `godep save ./...`
This should add the package to the vendor/ folder

# Packages Layout and Usages
- cmd
    - contains driver function, setup, and config loading
- pkg/controller
    - contains the core logic specific to escalator and nodegroups
- pkg/k8s
    - provides application agnostic helps for interfacing with Kubernetes and client-go
- pkg/cloudprovider
    - provides everything cloudprovider
- pkg/metrics
    - provides a place for all metric setup to live
- pkg/test
    - provides Kubernetes helpers for testing Kubernetes escalator code

# System Algorithm Design
The initial algorithm is designed to be as simple as possible to achieve our goal

## Very short summary
- The autoscaler is designed as a static autoscaler. 
- Start watching all pods and nodes in the cluster for a certain node group, including pending pods
- Every tick the autoscaler will check the overall cluster utilisation for each node group and make a decision whether to do nothing, scale up, or start making nodes unschedulable.
    - On a scale up event
        - untaint some configurable amount of nodes
        - try making MARKED nodes schedulable first
        - increase the ASG size by the remaining nodes (phase 3)
    - On a scale down event
        - taint some configurable amount of nodes
        - MARK (taint/cordon) the nodes so they become unschedulable
        - Set a grace period for the node (phase 3)
- A reaper routine will routinely check for MARKED nodes with no running pods and terminate them if the grace period has expired (phase 3)

![Algorithm](Algorithm.png)

# System Architecture
Basic idea of System Architecture. Subject to chage dramatically. Update image when design changes
![UML](UML.png)
