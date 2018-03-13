# Deployment

Escalator is able to be run both inside the cluster and out of the cluster, but it is highly recommended to run it
within the cluster. It is able to be run in the `kube-system` namespace.

## Cloud Providers<a name="cloud-provider"></a>

Escalator provides integration for the following cloud providers:

 - **AWS** - [see documentation](./aws/README.md)
   - Permissions
   - AWS Credentials
   - ASG Configuration
   - Common issues, caveats and gotchas
   
## Setup

### Kubernetes API Config

When running inside the cluster, Escalator will use the following for accessing the Kubernetes API:

```go
config, err := rest.InClusterConfig()
``` 

`rest.InClusterConfig()` uses the service account token inside the pod at 
`/var/run/secrets/kubernetes.io/serviceaccount` to gain access to the Kubernetes API. See 
[Authenticating inside the cluster](https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration).

Escalator will need certain permissions to list/patch/get/watch/update pods and nodes. See the section below on 
[RBAC](#rbac) to set up the service account, cluster role and cluster role binding.

To run Escalator outside of the cluster, use the `--kubeconfig=` flag to specify a path to a Kubernetes config. For
example, `--kubeconfig=~/.kube/config`.

### RBAC<a name="rbac"></a>

To be able to function correctly, Escalator needs a service account with the following permissions:

- **pods**: watch, list, get
- **nodes**: update, patch, watch, list, get
    
To create the service account, cluster role and cluster role binding, run the following:

```bash
kubectl create -f escalator-rbac.yaml
```

### ConfigMap

It is recommended to mount the `nodegroups_config.yaml` as a ConfigMap inside the pod for the node groups configuration.
 
To create the ConfigMap with an example `nodegroups_config.yaml` file, run the following:

```bash
kubectl create -f escalator-cm.yaml
```

### Deployment

This deployment makes use of the RBAC service account and ConfigMap created above.

To create the deployment, run the following:

```bash
kubectl create -f escalator-deployment.yaml
```

**See [Cloud Provider documentation](#cloud-provider) for deployments specific to a cloud provider.**
