package controller

import (
	time "github.com/stephanos/clock"
	duration "time"
	log "github.com/sirupsen/logrus"
	"fmt"
)

type scaleLock struct {
	isLocked            bool
	requestedNodes      int
	lockTime            duration.Time
	minimumLockDuration duration.Duration
	maximumLockDuration duration.Duration
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

func (l *scaleLock) timeUntilMinimumUnlock() duration.Duration {
	return l.lockTime.Add(l.minimumLockDuration).Sub(time.Now())
}

func (l *scaleLock) timeUntilMaximumUnlock() duration.Duration {
	return l.lockTime.Add(l.maximumLockDuration).Sub(time.Now())
}

func (l scaleLock) String() string {
	return fmt.Sprintf("%v before min cooldown. %v before lock timeout.",
		l.timeUntilMinimumUnlock(),
		l.timeUntilMaximumUnlock(),
	)
}
