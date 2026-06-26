package controller

import (
	"fmt"
	"time"

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
	cooldown         time.Duration
	nodegroup        string

	state               circuitState
	consecutiveFailures int
	lastRequestedTarget int64 // desired count requested at the last allowed scale-up
	sawScaleUp          bool  // an allowed scale-up is awaiting a fulfilment judgement
	openedAt            time.Time
}

func (b *scaleUpCircuitBreaker) enabled() bool {
	return b.failureThreshold > 0
}

// allow returns true if a cloud-provider desired-count increase is permitted.
// It judges the outcome of the previous allowed scale-up before deciding.
func (b *scaleUpCircuitBreaker) allow(currentSize int64) bool {
	if !b.enabled() {
		return true
	}

	if b.sawScaleUp {
		// The running count is expected to reach the desired count we requested
		// within the scale-up cooldown. If it has not, the cloud provider could
		// not deliver the capacity (e.g. AZ out of capacity), so count a failure.
		if currentSize >= b.lastRequestedTarget {
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
			b.openedAt = time.Now()
			metrics.NodeGroupScaleUpCircuitBreakerOpen.WithLabelValues(b.nodegroup).Set(1)
			log.WithField("nodegroup", b.nodegroup).
				Warnf("scale-up circuit breaker tripped after %d consecutive failures", b.consecutiveFailures)
			return false
		}
		return true
	case circuitOpen:
		if time.Since(b.openedAt) >= b.cooldown {
			b.state = circuitClosed
			// One further failure re-trips immediately.
			b.consecutiveFailures = b.failureThreshold - 1
			metrics.NodeGroupScaleUpCircuitBreakerOpen.WithLabelValues(b.nodegroup).Set(0)
			log.WithField("nodegroup", b.nodegroup).
				Infof("scale-up circuit breaker half-open probe: allowing one request after cooldown")
			return true
		}
		log.WithField("nodegroup", b.nodegroup).
			Debugf("scale-up circuit breaker still open, %v remaining", b.cooldown-time.Since(b.openedAt))
		return false
	}

	return true
}

// recordScaleUp notes the desired count we just requested so the next allow()
// call can judge whether the running count actually reached it.
func (b *scaleUpCircuitBreaker) recordScaleUp(requestedTarget int64) {
	b.lastRequestedTarget = requestedTarget
	b.sawScaleUp = true
}

func (b scaleUpCircuitBreaker) String() string {
	return fmt.Sprintf(
		"circuitBreaker(state=%v, consecutiveFailures=%d, lastRequestedTarget=%d, sawScaleUp=%v)",
		b.state,
		b.consecutiveFailures,
		b.lastRequestedTarget,
		b.sawScaleUp,
	)
}
