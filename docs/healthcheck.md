# Healthcheck

Escalator exposes a healthcheck endpoint at `/healthz`. When this endpoint is called, Escalator performs a "deep check"
on a number of components. These include:

 - **Cloud Provider**
    - Checks whether Escalator is able to refresh or describe the node group using the Cloud Provider's API
 - **Kubernetes API Server**
    - Checks whether Escalator has connectivity with the Kubernetes API server by listing nodes
    
## Usage

It is recommended to use Kubernetes' liveness probes to ensure that Escalator is running correctly. This can be
configured in the Escalator deployment pod spec:

```yaml
livenessProbe:
  httpGet:
    port: 8080
    path: /healthz
  initialDelaySeconds: 5
  periodSeconds: 5
```

The liveness probe can also be found in the [`escalator-deployment.yaml`](./deployment/escalator-deployment.yaml) spec.

Further reading on Liveness and Readiness probes can be found 
[here](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-probes/).
