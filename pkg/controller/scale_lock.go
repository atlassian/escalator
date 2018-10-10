package controller

import (
	"fmt"
	"time"

	"github.com/atlassian/escalator/pkg/metrics"

	log "github.com/sirupsen/logrus"
)

type scaleLock struct {
	isLocked            bool
	requestedNodes      int
	lockTime            time.Time
	minimumLockDuration time.Duration
	// Needed for metrics label value
	nodegroup           string
}

// locked returns whether the scale lock is locked
func (l *scaleLock) locked() bool {
	if time.Now().Sub(l.lockTime) < l.minimumLockDuration {
		return true
	}
	l.unlock()
	return l.isLocked
}

// lock locks the scale lock
func (l *scaleLock) lock(nodes int) {
	if l.isLocked {
		log.Warn("Scale lock already locked")
	}
	log.Debug("Locking scale lock")
	l.isLocked = true
	l.requestedNodes = nodes
	l.lockTime = time.Now()
}

// unlock unlocks the scale lock
func (l *scaleLock) unlock() {
	// Only if it's already locked, otherwise noop; handles frequent forced unlocking from the locked() call to avoid spurious metrics submission
	if l.isLocked {
		lockDuration := float64(time.Now().Sub(l.lockTime)) / 1000000000
		log.Debug(fmt.Sprintf("Unlocking scale lock. Lock Duration: %0.0e Node Group: %s", lockDuration, l.nodegroup))
		l.isLocked = false
		l.requestedNodes = 0
		metrics.NodeGroupScaleLockDuration.WithLabelValues(l.nodegroup).Observe(lockDuration)
	}
}

// timeUntilMinimumUnlock returns the the time until the minimum unlock
func (l *scaleLock) timeUntilMinimumUnlock() time.Duration {
	return l.lockTime.Add(l.minimumLockDuration).Sub(time.Now())
}

func (l scaleLock) String() string {
	return fmt.Sprintf(
		"lock(%v): there are %v upcoming nodes requested, %v before min cooldown.",
		l.locked(),
		l.requestedNodes,
		l.timeUntilMinimumUnlock(),
	)
}
