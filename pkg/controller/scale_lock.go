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
}

func (l *scaleLock) locked() bool {
	if time.Now().Sub(l.lockTime) < l.minimumLockDuration {
		return true
	}
	l.unlock()
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

func (l scaleLock) String() string {
	return fmt.Sprintf(
		"lock(%v): there are %v upcoming nodes requested, %v before min cooldown.",
		l.locked(),
		l.requestedNodes,
		l.timeUntilMinimumUnlock(),
	)
}
