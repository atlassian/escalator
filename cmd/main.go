package main

import (
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
	"k8s.io/client-go/kubernetes"

	log "github.com/sirupsen/logrus"
)

var (
	loglevel            = kingpin.Flag("loglevel", "Logging level passed into logrus. 4 for info, 5 for debug.").Short('v').Default(fmt.Sprintf("%d", log.InfoLevel)).Int()
	logfmt              = kingpin.Flag("logfmt", "Set the format of logging output. (json, ascii)").Default("ascii").Enum("ascii", "json")
	addr                = kingpin.Flag("address", "Address to listen to for /metrics").Default(":8080").String()
	scanInterval        = kingpin.Flag("scaninterval", "How often cluster is reevaluated for scale up or down").Default("60s").Duration()
	kubeConfigFile      = kingpin.Flag("kubeconfig", "Kubeconfig file location").String()
	nodegroupConfigFile = kingpin.Flag("nodegroups", "Config file for nodegroups").Required().String()
	drymode             = kingpin.Flag("drymode", "master drymode argument. If true, forces drymode on all nodegroups").Bool()
	cloudProviderID     = kingpin.Flag("cloud-provider", "Cloud provider to use. Available options: (aws)").Default("aws").Enum("aws")
	awsAssumeRoleARN    = kingpin.Flag("aws-assume-role-arn", "AWS role arn to assume. Only usable when using aws cloud provider. Example: arn:aws:iam::111111111111:role/escalator").String()
)

// cloudProviderBuilder builds the requested cloud provider. aws, gce, etc
type cloudProviderBuilder struct {
	ProviderOpts cloudprovider.BuildOpts
}

// Build builds the requested CloudProvider
func (b cloudProviderBuilder) Build() (cloudprovider.CloudProvider, error) {
	switch b.ProviderOpts.ProviderID {
	case aws.ProviderName:
		return aws.Builder{ProviderOpts: b.ProviderOpts, AssumeRoleARN: *awsAssumeRoleARN}.Build()
	default:
		err := fmt.Errorf("provider %v does not exist", b.ProviderOpts.ProviderID)
		log.Fatalln(err)
		return nil, err
	}
}

// setupCloudProvider creates the cloudprovider builder with the nodegroup opts
func setupCloudProvider(nodegroups []controller.NodeGroupOptions) cloudprovider.Builder {
	var nodegroupIDs []string
	for _, n := range nodegroups {
		nodegroupIDs = append(nodegroupIDs, n.CloudProviderASG)
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
			log.WithField("nodegroup", nodegroup.Name).Errorln("Validating options: [FAIL]")
			for _, err := range errs {
				log.WithError(err).Errorln("failed check")
			}
			log.WithField("nodegroup", nodegroup.Name).Fatalf("There are %v problems when validating the options. Please check %v", len(errs), *nodegroupConfigFile)
		}
		log.WithField("nodegroup", nodegroup.Name).Infoln("Validating options: [PASS]")
		log.WithField("nodegroup", nodegroup.Name).Infof("Registered with drymode %v", nodegroup.DryMode || *drymode)
	}

	return nodegroups
}

// setupK8SClient creates the incluster or out of cluster kubernetes config
func setupK8SClient() kubernetes.Interface {
	var k8sClient kubernetes.Interface

	// if the kubeConfigFile is in the cmdline args then use the out of cluster config
	if kubeConfigFile != nil && len(*kubeConfigFile) > 0 {
		log.Infoln("Using out of cluster config")
		k8sClient = k8s.NewOutOfClusterClient(*kubeConfigFile)
	} else {
		log.Infoln("Using in cluster config")
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
	log.Infoln("Stopping autoscaler gracefully")
	close(stopChan)
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

	log.Infoln("Starting with log level", log.GetLevel())

	nodegroups := setupNodeGroups()
	k8sClient := setupK8SClient()
	cloudBuilder := setupCloudProvider(nodegroups)

	// global stop channel. Close signal will be sent to broadvast a shutdown to everything waiting for it to stop
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
