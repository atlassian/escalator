package k8s

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	coordinationv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
)

// LeaderElectConfig stores the configuration for a leader election lock
type LeaderElectConfig struct {
	LeaseDuration time.Duration
	RenewDeadline time.Duration
	RetryPeriod   time.Duration
	Namespace     string
	Name          string
}

// GetLeaderElector returns a leader elector
func GetLeaderElector(ctx context.Context, config LeaderElectConfig, coreClient v1.CoreV1Interface, coordClient coordinationv1.CoordinationV1Interface, recorder record.EventRecorder, resourceLockID string) (*leaderelection.LeaderElector, context.Context, <-chan struct{}, error) {
	resourceLock, err := GetResourceLock(config.Namespace, config.Name, coreClient, coordClient, recorder, resourceLockID)
	if err != nil {
		return nil, nil, nil, err
	}

	ctxRet, cancel := context.WithCancel(ctx)
	startedLeading := make(chan struct{})
	le, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:          resourceLock,
		LeaseDuration: config.LeaseDuration,
		RenewDeadline: config.RenewDeadline,
		RetryPeriod:   config.RetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				log.WithFields(log.Fields{
					"lock":     resourceLock.Describe(),
					"identity": resourceLock.Identity(),
				}).Info("started leading")
				close(startedLeading)
			},
			OnStoppedLeading: func() {
				// The context being cancelled will trigger a handler that will
				// deal with being deposed.
				cancel()
			},
		},
	})

	return le, ctxRet, startedLeading, err
}

// GetResourceLock returns a resource lock for leader election
func GetResourceLock(ns string, name string, coreClient v1.CoreV1Interface, coordClient coordinationv1.CoordinationV1Interface, recorder record.EventRecorder, resourceLockID string) (resourcelock.Interface, error) {
	return resourcelock.New(
		resourcelock.LeasesResourceLock,
		ns,
		name,
		coreClient,
		coordClient,
		resourcelock.ResourceLockConfig{
			Identity:      resourceLockID,
			EventRecorder: recorder,
		})
}
