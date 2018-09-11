package k8s

import (
	"context"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes/typed/core/v1"
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
func GetLeaderElector(ctx context.Context, config LeaderElectConfig, client v1.CoreV1Interface, recorder record.EventRecorder) (*leaderelection.LeaderElector, context.Context, <-chan struct{}, error) {
	resourceLock, err := GetResourceLock(config.Namespace, config.Name, client, recorder)
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
			OnStartedLeading: func(stop <-chan struct{}) {
				log.WithFields(log.Fields{
					"lock":     resourceLock.Describe(),
					"identity": resourceLock.Identity(),
				}).Info("started leading")
				close(startedLeading)
			},
			OnStoppedLeading: func() {
				log.Warn("Leader status lost")
				cancel()
			},
		},
	})

	return le, ctxRet, startedLeading, err
}

// GetResourceLock returns a resource lock for leader election
func GetResourceLock(ns string, name string, client v1.CoreV1Interface, recorder record.EventRecorder) (resourcelock.Interface, error) {
	return resourcelock.New(
		resourcelock.ConfigMapsResourceLock,
		ns,
		name,
		client,
		resourcelock.ResourceLockConfig{
			Identity:      uuid.New().String(),
			EventRecorder: recorder,
		},
	)
}
