package controller

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newTestBreaker(threshold int, cooldown time.Duration) *scaleUpCircuitBreaker {
	return &scaleUpCircuitBreaker{
		failureThreshold: threshold,
		cooldown:         cooldown,
		nodegroup:        "test-ng",
	}
}

func TestCircuitBreakerDisabled(t *testing.T) {
	b := newTestBreaker(0, time.Minute)
	for i := 0; i < 10; i++ {
		assert.True(t, b.allow(int64(i)), "disabled breaker must always allow")
	}
}

func TestCircuitBreakerClosedResetsOnFulfilment(t *testing.T) {
	b := newTestBreaker(3, time.Minute)

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
	b := newTestBreaker(3, time.Minute)

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

func TestCircuitBreakerOpenBlocksUntilCooldown(t *testing.T) {
	b := newTestBreaker(2, 5*time.Minute)

	assert.True(t, b.allow(10))
	b.recordScaleUp(12)
	assert.True(t, b.allow(10)) // failure 1
	b.recordScaleUp(14)
	assert.False(t, b.allow(10)) // failure 2 → trips

	assert.False(t, b.allow(10))
	assert.False(t, b.allow(10))

	// Simulate cooldown elapsed by backdating openedAt.
	b.openedAt = time.Now().Add(-6 * time.Minute)

	assert.True(t, b.allow(10)) // half-open probe
	assert.Equal(t, circuitClosed, b.state)
}

func TestCircuitBreakerHalfOpenRetripsOnFailure(t *testing.T) {
	b := newTestBreaker(2, 5*time.Minute)

	assert.True(t, b.allow(10))
	b.recordScaleUp(12)
	assert.True(t, b.allow(10)) // failure 1
	b.recordScaleUp(14)
	assert.False(t, b.allow(10)) // failure 2 → trips

	b.openedAt = time.Now().Add(-6 * time.Minute)

	assert.True(t, b.allow(10)) // half-open probe
	assert.Equal(t, circuitClosed, b.state)
	assert.Equal(t, b.failureThreshold-1, b.consecutiveFailures)

	b.recordScaleUp(16)

	// Target still unmet → a single further failure re-trips immediately.
	assert.False(t, b.allow(10))
	assert.Equal(t, circuitOpen, b.state)
}

func TestCircuitBreakerHalfOpenRecovers(t *testing.T) {
	b := newTestBreaker(2, 5*time.Minute)

	assert.True(t, b.allow(10))
	b.recordScaleUp(12)
	assert.True(t, b.allow(10)) // failure 1
	b.recordScaleUp(14)
	assert.False(t, b.allow(10)) // failure 2 → trips

	b.openedAt = time.Now().Add(-6 * time.Minute)

	assert.True(t, b.allow(10)) // half-open probe
	b.recordScaleUp(16)

	// Capacity recovered: running count reached the requested target.
	assert.True(t, b.allow(16))
	assert.Equal(t, circuitClosed, b.state)
	assert.Equal(t, 0, b.consecutiveFailures)
}

func TestCircuitBreakerFailedScaleEventCounter(t *testing.T) {
	b := newTestBreaker(5, time.Minute)

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
