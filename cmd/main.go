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
	"gopkg.in/alecthomas/kingpin.v2"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"

	log "github.com/sirupsen/logrus"
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
	leaderElectConfigNamespace = kingpin.Flag("leader-elect-config-namespace", "Leader election config map namespace").Default("kube-system").String()
	leaderElectConfigName      = kingpin.Flag("leader-elect-config-name", "Leader election config map name").Default("escalator-leader-elect").String()
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
		err := fmt.Errorf("provider %v does not exist", b.ProviderOpts.ProviderID)
		log.Fatal(err)
		return nil, err
	}
}

// setupCloudProvider creates the cloudprovider builder with the nodegroup opts
func setupCloudProvider(nodegroups []controller.NodeGroupOptions) cloudprovider.Builder {
	var nodegroupIDs []string
	for _, n := range nodegroups {
		nodegroupIDs = append(nodegroupIDs, n.CloudProviderGroupName)
	}
	cloudBuilder := cloudProviderBuilder{
		ProviderOpts: cloudprovider.BuildOpts{
			ProviderID:   *cloudProviderID,
			NodeGroupIDs: nodegroupIDs,
		},
	}
	return cloudBuilder
}

// setupNodeGroups reads and validates the nodegroupoptions
func setupNodeGroups() []controller.NodeGroupOptions {
	// nodegroupConfigFile is required by kingpin. Won't get to here if it's not defined
	configFile, err := os.Open(*nodegroupConfigFile)
	if err != nil {
		log.Fatalf("Failed to open configFile: %v", err)
	}
	nodegroups, err := controller.UnmarshalNodeGroupOptions(configFile)
	if err != nil {
		log.Fatalf("Failed to decode configFile: %v", err)
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

	return nodegroups
}

// setupK8SClient creates the incluster or out of cluster kubernetes config
func setupK8SClient(kubeConfigFile *string, leaderElect *bool) kubernetes.Interface {
	var k8sClient kubernetes.Interface

	// if the kubeConfigFile is in the cmdline args then use the out of cluster config
	if kubeConfigFile != nil && len(*kubeConfigFile) > 0 {
		log.Info("Using out of cluster config")
		if *leaderElect {
			log.Warn("Doing leader election out of cluster is not recommended.")
		}
		k8sClient = k8s.NewOutOfClusterClient(*kubeConfigFile)
	} else {
		log.Info("Using in cluster config")
		k8sClient = k8s.NewInClusterClient()
	}

	return k8sClient
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
func startLeaderElection(client kubernetes.Interface, config k8s.LeaderElectConfig) (context.Context, error) {
	eventsScheme := runtime.NewScheme()
	if err := coreV1.AddToScheme(eventsScheme); err != nil {
		return nil, err
	}

	// Start events recorder
	eventBroadcaster := record.NewBroadcaster()
	recorder := eventBroadcaster.NewRecorder(eventsScheme, coreV1.EventSource{Component: "escalator"})

	// Create leader elector
	leaderElector, ctx, startedLeading, err := k8s.GetLeaderElector(context.Background(), config, client.CoreV1(), recorder)
	if err != nil {
		return nil, err
	}

	go leaderElector.Run()
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
		log.Fatalf("Invalid log level %v provided. Must be between 0 (Critical) and 5 (Debug)", *loglevel)
	}
	log.SetLevel(log.Level(*loglevel))

	if *logfmt == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	}

	log.Info("Starting with log level", log.GetLevel())

	nodegroups := setupNodeGroups()
	k8sClient := setupK8SClient(kubeConfigFile, leaderElect)
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

	// If leader election is enabled, do leader election or die
	if *leaderElect {
		leaderContext, err := startLeaderElection(k8sClient, k8s.LeaderElectConfig{
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

	// start serving metrics endpoint
	metrics.Start(*addr)

	// create the controller and run in a loop until the stop signal
	opts := controller.Opts{
		ScanInterval:         *scanInterval,
		K8SClient:            k8sClient,
		NodeGroups:           nodegroups,
		DryMode:              *drymode,
		CloudProviderBuilder: cloudBuilder,
	}
	c := controller.NewController(opts, stopChan)
	c.RunForever(true)
}
