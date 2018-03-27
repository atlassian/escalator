package controller

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

type scaleLock struct {
	isLocked            bool
	requestedNodes      int
	lockTime            time.Time
	minimumLockDuration time.Duration
	maximumLockDuration time.Duration
}

func (l *scaleLock) locked() bool {
	if time.Now().Sub(l.lockTime) < l.minimumLockDuration {
		log.Debugln("Locked, have not exceeded minimum lock duration")
		return true
	}
	if time.Now().Sub(l.lockTime) > l.maximumLockDuration {
		l.unlock()
	}
	return l.isLocked
}

func (l *scaleLock) lock(nodes int) {
	log.Debugln("Locking scale lock")
	l.isLocked = true
	l.requestedNodes = nodes
	l.lockTime = time.Now()
}

func (l *scaleLock) unlock() {
	log.Debugln("Unlocking scale lock")
	l.isLocked = false
	l.requestedNodes = 0
}

func (l *scaleLock) timeUntilMinimumUnlock() time.Duration {
	return l.lockTime.Add(l.minimumLockDuration).Sub(time.Now())
}

func (l *scaleLock) timeUntilMaximumUnlock() time.Duration {
	return l.lockTime.Add(l.maximumLockDuration).Sub(time.Now())
}

func (l scaleLock) String() string {
	return fmt.Sprintf("%v before min cooldown. %v before lock timeout.",
		l.timeUntilMinimumUnlock(),
		l.timeUntilMaximumUnlock(),
	)
}
