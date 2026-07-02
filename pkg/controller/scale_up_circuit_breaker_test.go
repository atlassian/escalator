package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestBreaker(threshold int) *scaleUpCircuitBreaker {
	return &scaleUpCircuitBreaker{
		failureThreshold: threshold,
		nodegroup:        "test-ng",
	}
}

func TestCircuitBreakerDisabled(t *testing.T) {
	b := newTestBreaker(0)
	for i := 0; i < 10; i++ {
		assert.True(t, b.allow(int64(i)), "disabled breaker must always allow")
	}
}

func TestCircuitBreakerClosedResetsOnFulfilment(t *testing.T) {
	b := newTestBreaker(3)

	// First call has no prior request to judge.
	assert.True(t, b.allow(10))
	b.recordScaleUp(12)

	// Running count reached the requested target: failure count resets.
	assert.True(t, b.allow(12))
	assert.Equal(t, 0, b.consecutiveFailures)
	b.recordScaleUp(14)

	assert.True(t, b.allow(14))
	assert.Equal(t, 0, b.consecutiveFailures)
}

func TestCircuitBreakerTripsAfterThreshold(t *testing.T) {
	b := newTestBreaker(3)

	// Running count is stuck at 10 while each request raises the target.
	assert.True(t, b.allow(10))
	b.recordScaleUp(12)
	assert.True(t, b.allow(10)) // 10 < 12 → failure 1
	b.recordScaleUp(14)
	assert.True(t, b.allow(10)) // failure 2
	b.recordScaleUp(16)
	assert.False(t, b.allow(10)) // failure 3 → trips
	assert.Equal(t, circuitOpen, b.state)

	assert.False(t, b.allow(10))
}

func TestCircuitBreakerStaysOpenUntilRunningReachesTarget(t *testing.T) {
	b := newTestBreaker(2)

	assert.True(t, b.allow(10))
	b.recordScaleUp(12)
	assert.True(t, b.allow(10)) // failure 1
	b.recordScaleUp(14)
	assert.False(t, b.allow(10)) // failure 2 → trips, desired frozen at 14
	assert.Equal(t, circuitOpen, b.state)

	// Running count still below the frozen target: stays open, no probing.
	assert.False(t, b.allow(11))
	assert.False(t, b.allow(13))
	assert.Equal(t, circuitOpen, b.state)
}

func TestCircuitBreakerRecoversWhenRunningCatchesUp(t *testing.T) {
	b := newTestBreaker(2)

	assert.True(t, b.allow(10))
	b.recordScaleUp(12)
	assert.True(t, b.allow(10)) // failure 1
	b.recordScaleUp(14)
	assert.False(t, b.allow(10)) // failure 2 → trips, desired frozen at 14

	// Cloud provider finally delivers the capacity: running reaches the frozen
	// target, so the breaker closes and scaling resumes.
	assert.True(t, b.allow(14))
	assert.Equal(t, circuitClosed, b.state)
	assert.Equal(t, 0, b.consecutiveFailures)

	// Normal scaling continues from here.
	b.recordScaleUp(16)
	assert.True(t, b.allow(16))
	assert.Equal(t, 0, b.consecutiveFailures)
}

func TestCircuitBreakerFailedScaleEventCounter(t *testing.T) {
	b := newTestBreaker(5)

	assert.True(t, b.allow(10))
	b.recordScaleUp(12)

	for i := 0; i < 3; i++ {
		b.allow(10) // 10 < 12 → failure
		if i < 2 {
			b.recordScaleUp(12)
		}
	}
	assert.Equal(t, 3, b.consecutiveFailures)
}
