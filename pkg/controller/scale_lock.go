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
	nodegroup string
}

// locked returns whether the scale lock is locked
func (l *scaleLock) locked() bool {
	if time.Since(l.lockTime) < l.minimumLockDuration {
		metrics.NodeGroupScaleLockCheckWasLocked.WithLabelValues(l.nodegroup).Add(1.0)
		return true
	}
	l.unlock()
	return l.isLocked
}

// lock locks the scale lock
func (l *scaleLock) lock(nodes int) {
	// Using `Add` instead of `Set` to catch locking when already locked
	metrics.NodeGroupScaleLock.WithLabelValues(l.nodegroup).Add(1.0)
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
		// Recording the lock duration in seconds, if $cloud provider could do scaling in nanosecond resolution; good problem to have.
		lockDuration := time.Since(l.lockTime).Seconds()
		log.Debug(fmt.Sprintf("Unlocking scale lock. Lock Duration: %0.0f s Node Group: %s", lockDuration, l.nodegroup))
		l.isLocked = false
		l.requestedNodes = 0
		metrics.NodeGroupScaleLockDuration.WithLabelValues(l.nodegroup).Observe(lockDuration)
		metrics.NodeGroupScaleLock.WithLabelValues(l.nodegroup).Set(0.0)
	}
}

// timeUntilMinimumUnlock returns the the time until the minimum unlock
func (l *scaleLock) timeUntilMinimumUnlock() time.Duration {
	return time.Until(l.lockTime.Add(l.minimumLockDuration))
}

func (l scaleLock) String() string {
	return fmt.Sprintf(
		"lock(%v): there are %v upcoming nodes requested, %v before min cooldown.",
		l.locked(),
		l.requestedNodes,
		l.timeUntilMinimumUnlock(),
	)
}
