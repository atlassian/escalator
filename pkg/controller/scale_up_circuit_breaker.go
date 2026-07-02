package controller

import (
	"github.com/atlassian/escalator/pkg/metrics"
	log "github.com/sirupsen/logrus"
)

type circuitState int

const (
	circuitClosed circuitState = iota
	circuitOpen
)

func (s circuitState) String() string {
	switch s {
	case circuitClosed:
		return "closed"
	case circuitOpen:
		return "open"
	default:
		return "unknown"
	}
}

type scaleUpCircuitBreaker struct {
	failureThreshold int
	nodegroup        string

	state               circuitState
	consecutiveFailures int
	lastObservedSize    int64 // running count observed at the last allowed scale-up
	sawScaleUp          bool  // an allowed scale-up is awaiting a fulfilment judgement
}

// newScaleUpCircuitBreaker builds a breaker for a node group. When enabled it
// seeds the "open" gauge to 0 so dashboards have a baseline line to sit on even
// for a group that never trips.
func newScaleUpCircuitBreaker(nodegroup string, failureThreshold int) scaleUpCircuitBreaker {
	b := scaleUpCircuitBreaker{
		failureThreshold: failureThreshold,
		nodegroup:        nodegroup,
	}
	if b.enabled() {
		metrics.NodeGroupScaleUpCircuitBreakerOpen.WithLabelValues(nodegroup).Set(0)
	}
	return b
}

func (b *scaleUpCircuitBreaker) enabled() bool {
	return b.failureThreshold > 0
}

// allow reports whether a cloud-provider desired-count increase is permitted.
// currentSize is the running count (the ASG actual size) and targetSize is the
// current desired count. It judges the outcome of the previous allowed scale-up
// before deciding.
//
// A scale-up is judged a failure only when the running count has not grown since
// the previous allowed scale-up: the cloud provider delivered no new capacity
// (e.g. an AZ is out of capacity). A group whose running count is still climbing,
// even slowly and even while desired keeps rising, is considered healthy and does
// not accrue failures.
//
// The breaker has no cooldown timer. It keeps looping as normal while open and
// simply refuses to raise the desired count until the running count catches up to
// the current desired count. Once the cloud provider delivers that capacity, the
// breaker closes and normal scaling resumes.
func (b *scaleUpCircuitBreaker) allow(currentSize, targetSize int64) bool {
	if !b.enabled() {
		return true
	}

	if b.sawScaleUp {
		if currentSize > b.lastObservedSize {
			// Running count grew since the last scale-up: the cloud provider is
			// delivering capacity, so reset the failure count.
			b.consecutiveFailures = 0
		} else {
			b.consecutiveFailures++
			metrics.NodeGroupScaleUpFailedScaleEvents.WithLabelValues(b.nodegroup).Add(1)
		}
		b.sawScaleUp = false
	}

	switch b.state {
	case circuitClosed:
		if b.consecutiveFailures >= b.failureThreshold {
			b.state = circuitOpen
			metrics.NodeGroupScaleUpCircuitBreakerOpen.WithLabelValues(b.nodegroup).Set(1)
			log.WithField("nodegroup", b.nodegroup).
				Warnf("scale-up circuit breaker tripped after %d consecutive failures", b.consecutiveFailures)
			return false
		}
		return true
	case circuitOpen:
		// Recovery is signalled by the running count catching up to the current
		// desired count. No probing or waiting required: while open the desired
		// count is held steady, so once the cloud provider delivers that capacity
		// the running count catches up and we resume. Comparing against the live
		// desired count (rather than a value frozen at trip time) means an external
		// scale-down or a manual reset of the desired count also lets the breaker
		// recover, instead of leaving it stuck open until a restart.
		if currentSize >= targetSize {
			b.state = circuitClosed
			b.consecutiveFailures = 0
			metrics.NodeGroupScaleUpCircuitBreakerOpen.WithLabelValues(b.nodegroup).Set(0)
			log.WithField("nodegroup", b.nodegroup).
				Infof("scale-up circuit breaker closed: running count caught up to desired, resuming scaling")
			return true
		}
		log.WithField("nodegroup", b.nodegroup).
			Debugf("scale-up circuit breaker still open: running %v has not reached desired %v",
				currentSize, targetSize)
		return false
	}

	return true
}

// recordScaleUp notes the running count at the moment of an allowed scale-up so
// the next allow() call can tell whether the cloud provider delivered any new
// capacity in the interim.
func (b *scaleUpCircuitBreaker) recordScaleUp(observedSize int64) {
	b.lastObservedSize = observedSize
	b.sawScaleUp = true
}
