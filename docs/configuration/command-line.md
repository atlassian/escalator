# Command line options

Use `--help` to get the full usage info:

```
$ escalator --help
usage: escalator --nodegroups=NODEGROUPS [<flags>]

Flags:
      --help                   Show context-sensitive help (also try --help-long and --help-man).
  -v, --loglevel=4             Logging level passed into logrus. 4 for info, 5 for debug.
      --logfmt=ascii           Set the format of logging output. (json, ascii)
      --address=":8080"        Address to listen to for /metrics
      --scaninterval=60s       How often cluster is reevaluated for scale up or down
      --kubeconfig=KUBECONFIG  Kubeconfig file location
      --nodegroups=NODEGROUPS  Config file for nodegroups
      --drymode                master drymode argument. If true, forces drymode on all nodegroups
      --cloud-provider=aws     Cloud provider to use. Available options: (aws)
      --aws-assume-role-arn=AWS-ASSUME-ROLE-ARN
                               AWS role arn to assume. Only usable when using the aws cloud provider. Example: arn:aws:iam::111111111111:role/escalator
      --leader-elect           Enable leader election
      --leader-elect-lease-duration=15s
                               Leader election lease duration
      --leader-elect-renew-deadline=10s
                               Leader election renew deadline
      --leader-elect-retry-period=2s
                               Leader election retry period
      --leader-elect-config-namespace="kube-system"
                               Leader election lease object namespace
      --leader-elect-config-name="escalator-leader-elect"
                               Leader election lease object name
```

## Options

### `-v, --loglevel`

Determines the log level for Escalator. [logrus](https://github.com/sirupsen/logrus) is being used to handle log format
and logging levels. You can see the logrus logging levels [here](https://github.com/sirupsen/logrus#level-logging).

In some situations it may be helpful to disable debug logging as it can be quite verbose.

#### Examples:

- `-v 5` show all log levels, including debug
- `-v 4` show info log level and above
- `-v 1` only show fatal log level and above

### `--logfmt`

Defines the log format

#### ascii

```
INFO[0000] Starting with log level info                 
INFO[0000] Validating options: [PASS]                    nodegroup=shared
INFO[0000] Registered with drymode false                 nodegroup=shared
INFO[0000] Using in cluster config          
```

#### json

```json
{"level":"info","msg":"Starting with log level debug","time":"2018-03-09T16:53:33+11:00"}
{"level":"info","msg":"Validating options: [PASS]","nodegroup":"shared","time":"2018-03-09T16:53:33+11:00"}
{"level":"info","msg":"Registered with drymode false","nodegroup":"shared","time":"2018-03-09T16:53:33+11:00"}
{"level":"info","msg":"Using in cluster config","time":"2018-03-09T16:53:33+11:00"}
```

### `--address`

Address to listen on for `/metrics` and `/healthz`. Must be in a format that 
[http.ListenAndServe](https://golang.org/pkg/net/http/#ListenAndServe) can interpret.

### `--scaninterval`

How often to perform a scan or run. It is recommended to have this configured between 30 seconds to 60 seconds.
Too long of a scan interval can lead to Escalator reacting too slow to scaling up the cluster. 
Too short of a scan interval can lead to to Escalator scaling too quickly and imprecisely.

### `--kubeconfig`

The path to the config that [client-go](https://github.com/kubernetes/client-go) uses for connecting to Kubernetes.
Note: this isn't required when running Escalator inside the cluster as Escalator will get it's credentials from 
the Kubernetes environment variables.

### `--nodegroups`

The path to the nodegroups yaml config file that defines the node groups and options. Full nodegroups configuration
can be found here.

### `--drymode`

Master drymode flag to force "dry mode" on all node groups. Dry mode will log the actions that Escalator will perform
without actually running them.

### `--cloud-provider`

The cloud provider to use. Cloud provider configuration can be found [here](../deployment/README.md).

### `--aws-assume-role-arn`

Provides an option to specify an AWS IAM role to assume when Escalator starts. **Only works with AWS Cloud Provider.**

### `--leader-elect`

Enable leader election behaviour. Note that Escalator uses a ConfigMap for the leader lock, not an Endpoint.

### `--leader-elect-lease-duration`

Sets how long a nonleader will wait before it attempts to require the leadership. Measured against time of last observed ack.

### `--leader-elect-renew-deadline`

Sets how long an acting leader will retry refreshing leadership before giving up.

### `--leader-elect-retry-period`

Sets how long all the clients will wait in between attempts of any action.

### `--leader-elect-config-namespace`

Sets the namespace where the lease object used for locking will be created or looked for.

### `--leader-elect-config-name`

Sets the name of the lease object used for locking.