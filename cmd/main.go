package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/atlassian/escalator/pkg/cloudprovider"
	"github.com/atlassian/escalator/pkg/cloudprovider/aws"
	"github.com/atlassian/escalator/pkg/controller"
	"github.com/atlassian/escalator/pkg/k8s"
	"github.com/atlassian/escalator/pkg/metrics"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
	coordinationv1 "k8s.io/api/coordination/v1"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
)

var (
	loglevel                   = kingpin.Flag("loglevel", "Logging level passed into logrus. 4 for info, 5 for debug.").Short('v').Default(fmt.Sprintf("%d", log.InfoLevel)).Int()
	logfmt                     = kingpin.Flag("logfmt", "Set the format of logging output. (json, ascii)").Default("ascii").Enum("ascii", "json")
	addr                       = kingpin.Flag("address", "Address to listen to for /metrics").Default(":8080").String()
	scanInterval               = kingpin.Flag("scaninterval", "How often cluster is reevaluated for scale up or down").Default("60s").Duration()
	kubeConfigFile             = kingpin.Flag("kubeconfig", "Kubeconfig file location").String()
	nodegroupConfigFile        = kingpin.Flag("nodegroups", "Config file for nodegroups").Required().String()
	drymode                    = kingpin.Flag("drymode", "master drymode argument. If true, forces drymode on all nodegroups").Bool()
	cloudProviderID            = kingpin.Flag("cloud-provider", "Cloud provider to use. Available options: (aws)").Default("aws").Enum("aws")
	awsAssumeRoleARN           = kingpin.Flag("aws-assume-role-arn", "AWS role arn to assume. Only usable when using the aws cloud provider. Example: arn:aws:iam::111111111111:role/escalator").String()
	leaderElect                = kingpin.Flag("leader-elect", "Enable leader election").Default("false").Bool()
	leaderElectLeaseDuration   = kingpin.Flag("leader-elect-lease-duration", "Leader election lease duration").Default("15s").Duration()
	leaderElectRenewDeadline   = kingpin.Flag("leader-elect-renew-deadline", "Leader election renew deadline").Default("10s").Duration()
	leaderElectRetryPeriod     = kingpin.Flag("leader-elect-retry-period", "Leader election retry period").Default("2s").Duration()
	leaderElectConfigNamespace = kingpin.Flag("leader-elect-config-namespace", "Leader election lease object  namespace").Default("kube-system").String()
	leaderElectConfigName      = kingpin.Flag("leader-elect-config-name", "Leader election lease object name").Default("escalator-leader-elect").String()
)

// cloudProviderBuilder builds the requested cloud provider. aws, gce, etc
type cloudProviderBuilder struct {
	ProviderOpts cloudprovider.BuildOpts
}

// Build builds the requested CloudProvider
func (b cloudProviderBuilder) Build() (cloudprovider.CloudProvider, error) {
	switch b.ProviderOpts.ProviderID {
	case aws.ProviderName:
		return aws.Builder{
			ProviderOpts: b.ProviderOpts,
			Opts: aws.Opts{
				AssumeRoleARN: *awsAssumeRoleARN,
			},
		}.Build()
	default:
		return nil, errors.Errorf("provider %v does not exist", b.ProviderOpts.ProviderID)
	}
}

// setupCloudProvider creates the cloudprovider builder with the nodegroup opts
func setupCloudProvider(nodegroups []controller.NodeGroupOptions) cloudprovider.Builder {
	var nodeGroupConfigs []cloudprovider.NodeGroupConfig
	for _, n := range nodegroups {
		nodeGroupConfigs = append(nodeGroupConfigs, cloudprovider.NodeGroupConfig{
			Name:    n.Name,
			GroupID: n.CloudProviderGroupName,
			AWSConfig: cloudprovider.AWSNodeGroupConfig{
				LaunchTemplateID:          n.AWS.LaunchTemplateID,
				LaunchTemplateVersion:     n.AWS.LaunchTemplateVersion,
				FleetInstanceReadyTimeout: n.AWS.FleetInstanceReadyTimeoutDuration(),
				Lifecycle:                 n.AWS.Lifecycle,
				InstanceTypeOverrides:     n.AWS.InstanceTypeOverrides,
				ResourceTagging:           n.AWS.ResourceTagging,
			},
		})
	}
	cloudBuilder := cloudProviderBuilder{
		ProviderOpts: cloudprovider.BuildOpts{
			ProviderID:       *cloudProviderID,
			NodeGroupConfigs: nodeGroupConfigs,
		},
	}
	return cloudBuilder
}

// setupNodeGroups reads and validates the nodegroupoptions
func setupNodeGroups() ([]controller.NodeGroupOptions, error) {
	// nodegroupConfigFile is required by kingpin. Won't get to here if it's not defined
	configFile, err := os.Open(*nodegroupConfigFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open configFile")
	}
	nodegroups, err := controller.UnmarshalNodeGroupOptions(configFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode configFile")
	}

	// Validate each nodegroup options
	for _, nodegroup := range nodegroups {
		errs := controller.ValidateNodeGroup(nodegroup)
		if len(errs) > 0 {
			log.WithField("nodegroup", nodegroup.Name).Error("Validating options: [FAIL]")
			for _, err := range errs {
				log.WithError(err).Error("failed check")
			}
			log.WithField("nodegroup", nodegroup.Name).Fatalf("There are %v problems when validating the options. Please check %v", len(errs), *nodegroupConfigFile)
		}
		log.WithField("nodegroup", nodegroup.Name).Info("Validating options: [PASS]")
		log.WithField("nodegroup", nodegroup.Name).Infof("Registered with drymode %v", nodegroup.DryMode || *drymode)
	}

	return nodegroups, nil
}

// setupK8SClient creates the incluster or out of cluster kubernetes config
func setupK8SClient(kubeConfigFile *string, leaderElect *bool) (kubernetes.Interface, error) {
	// if the kubeConfigFile is in the cmdline args then use the out of cluster config
	if kubeConfigFile != nil && len(*kubeConfigFile) > 0 {
		log.Info("Using out of cluster config")
		if *leaderElect {
			log.Warn("Doing leader election out of cluster is not recommended.")
		}
		return k8s.NewOutOfClusterClient(*kubeConfigFile)
	}
	log.Info("Using in cluster config")
	return k8s.NewInClusterClient()
}

// awaitStopSignal awaits termination signals and shutdown gracefully
func awaitStopSignal(stopChan chan struct{}) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-signalChan

	log.Infof("Signal received: %v", sig)
	log.Info("Stopping autoscaler gracefully")
	close(stopChan)
}

func awaitLeaderDeposed(leaderContext context.Context) {
	// If the leader Context is finished, that's because we stopped leading.
	// so we will crash.
	<-leaderContext.Done()

	log.Fatal("Was deposed as leader, exiting.")

}

// startLeaderElection creates and starts the leader election
func startLeaderElection(client kubernetes.Interface, resourceLockID string, config k8s.LeaderElectConfig) (context.Context, error) {
	eventsScheme := runtime.NewScheme()
	if err := coordinationv1.AddToScheme(eventsScheme); err != nil {
		return nil, err
	}
	if err := coreV1.AddToScheme(eventsScheme); err != nil {
		return nil, err
	}

	// Start events recorder and get it logging and recording.
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Infof)
	eventBroadcaster.StartRecordingToSink(&clientcorev1.EventSinkImpl{Interface: clientcorev1.New(client.CoreV1().RESTClient()).Events("")})
	recorder := eventBroadcaster.NewRecorder(eventsScheme, coreV1.EventSource{Component: "escalator"})

	// Create leader elector
	leaderElector, ctx, startedLeading, err := k8s.GetLeaderElector(context.Background(), config, client.CoreV1(), client.CoordinationV1(), recorder, resourceLockID)
	if err != nil {
		return nil, err
	}

	go leaderElector.Run(ctx)
	select {
	case <-ctx.Done():
		return ctx, ctx.Err()
	case <-startedLeading:
		return ctx, nil
	}
}

func main() {

	kingpin.Parse()

	// setup logging
	if *loglevel < 0 || *loglevel > 5 {
		fmt.Fprintf(os.Stderr, "Invalid log level %v provided. Must be between 0 (Critical) and 5 (Debug)\n", *loglevel)
		os.Exit(1)
	}
	log.SetLevel(log.Level(*loglevel))

	if *logfmt == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	}

	log.Info("Starting with log level", log.GetLevel())

	nodegroups, err := setupNodeGroups()
	if err != nil {
		log.Fatal(err)
	}
	k8sClient, err := setupK8SClient(kubeConfigFile, leaderElect)
	if err != nil {
		log.Fatal(err)
	}
	cloudBuilder := setupCloudProvider(nodegroups)

	// Thanks to the Kube client's use of glog, and glog's requirement to run
	// flag.Parse() before logging anything, we need to run flag.Parse here.
	// But, it will conflict with the Kingpin flag parsing unless we mess with
	// os.Args. So, we'll save it out, run flag.Parse() and then put it back in case
	// else needs it. Yeah, I feel as dirty writing this as you do reading it.
	// Suggestions on how to avoid this problem welcomed.
	tempArgs := os.Args
	os.Args = []string{"escalator"}
	flag.Parse()
	os.Args = tempArgs

	// start serving metrics endpoint
	metrics.Start(*addr)

	// If leader election is enabled, do leader election or die
	if *leaderElect {
		// Having the resource lock ID be the pod name makes the configmap more human-readable.
		// Use a UUID as the failure case.
		var resourceLockID string
		resourceLockID, isPodNameEnvSet := os.LookupEnv("POD_NAME")
		if !isPodNameEnvSet {
			resourceLockID = uuid.New().String()
		}

		leaderContext, err := startLeaderElection(k8sClient, resourceLockID, k8s.LeaderElectConfig{
			LeaseDuration: *leaderElectLeaseDuration,
			RenewDeadline: *leaderElectRenewDeadline,
			RetryPeriod:   *leaderElectRetryPeriod,
			Namespace:     *leaderElectConfigNamespace,
			Name:          *leaderElectConfigName,
		})
		if err != nil {
			log.WithError(err).Fatal("Leader election returned an error")
		}
		go awaitLeaderDeposed(leaderContext)
	}

	// global stop channel. Close signal will be sent to broadcast a shutdown to everything waiting for it to stop
	stopChan := make(chan struct{}, 1)
	go awaitStopSignal(stopChan)

	// create the controller and run in a loop until the stop signal
	opts := controller.Opts{
		ScanInterval:         *scanInterval,
		K8SClient:            k8sClient,
		NodeGroups:           nodegroups,
		DryMode:              *drymode,
		CloudProviderBuilder: cloudBuilder,
	}
	c, err := controller.NewController(opts, stopChan)
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(c.RunForever(true))
}
