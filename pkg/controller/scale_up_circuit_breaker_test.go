package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestBreaker(threshold int) *scaleUpCircuitBreaker {
	b := newScaleUpCircuitBreaker("test-ng", threshold)
	return &b
}

func TestCircuitBreakerDisabled(t *testing.T) {
	b := newTestBreaker(0)
	for i := 0; i < 10; i++ {
		assert.True(t, b.allow(int64(i), int64(i+5)), "disabled breaker must always allow")
	}
}

func TestCircuitBreakerResetsWhenRunningGrows(t *testing.T) {
	b := newTestBreaker(3)

	// First call has no prior scale-up to judge.
	assert.True(t, b.allow(10, 12))
	b.recordScaleUp(10)

	// Running count grew since the last scale-up: capacity is being delivered, so
	// the failure count resets.
	assert.True(t, b.allow(11, 14))
	assert.Equal(t, 0, b.consecutiveFailures)
	b.recordScaleUp(11)

	assert.True(t, b.allow(12, 16))
	assert.Equal(t, 0, b.consecutiveFailures)
}

// A busy cluster whose desired count rises faster than instances launch is still
// healthy as long as the running count keeps climbing, and must not trip.
func TestCircuitBreakerDoesNotTripWhenRunningKeepsGrowing(t *testing.T) {
	b := newTestBreaker(2)

	assert.True(t, b.allow(10, 15))
	b.recordScaleUp(10)
	assert.True(t, b.allow(11, 20)) // running 10 -> 11, progress
	b.recordScaleUp(11)
	assert.True(t, b.allow(12, 25)) // running 11 -> 12, progress
	b.recordScaleUp(12)
	assert.True(t, b.allow(13, 30)) // running 12 -> 13, progress

	assert.Equal(t, circuitClosed, b.state)
	assert.Equal(t, 0, b.consecutiveFailures)
}

func TestCircuitBreakerTripsAfterThreshold(t *testing.T) {
	b := newTestBreaker(3)

	// Running count is stuck at 10 while each scale-up raises the desired count.
	assert.True(t, b.allow(10, 12))
	b.recordScaleUp(10)
	assert.True(t, b.allow(10, 14)) // running didn't grow -> failure 1
	b.recordScaleUp(10)
	assert.True(t, b.allow(10, 16)) // failure 2
	b.recordScaleUp(10)
	assert.False(t, b.allow(10, 18)) // failure 3 -> trips
	assert.Equal(t, circuitOpen, b.state)

	assert.False(t, b.allow(10, 18))
}

func TestCircuitBreakerStaysOpenUntilRunningReachesDesired(t *testing.T) {
	b := newTestBreaker(2)

	assert.True(t, b.allow(10, 12))
	b.recordScaleUp(10)
	assert.True(t, b.allow(10, 14)) // failure 1
	b.recordScaleUp(10)
	assert.False(t, b.allow(10, 16)) // failure 2 -> trips, desired 16
	assert.Equal(t, circuitOpen, b.state)

	// Running count still below desired: stays open, no probing.
	assert.False(t, b.allow(11, 16))
	assert.False(t, b.allow(15, 16))
	assert.Equal(t, circuitOpen, b.state)
}

func TestCircuitBreakerRecoversWhenRunningCatchesUp(t *testing.T) {
	b := newTestBreaker(2)

	assert.True(t, b.allow(10, 12))
	b.recordScaleUp(10)
	assert.True(t, b.allow(10, 14)) // failure 1
	b.recordScaleUp(10)
	assert.False(t, b.allow(10, 16)) // failure 2 -> trips, desired 16

	// Cloud provider finally delivers the capacity: running reaches desired, so
	// the breaker closes and scaling resumes.
	assert.True(t, b.allow(16, 16))
	assert.Equal(t, circuitClosed, b.state)
	assert.Equal(t, 0, b.consecutiveFailures)

	// Normal scaling continues from here.
	b.recordScaleUp(16)
	assert.True(t, b.allow(18, 20))
	assert.Equal(t, 0, b.consecutiveFailures)
}

// Comparing against the live desired count (not a value frozen at trip time)
// means the breaker recovers if desired is lowered externally while it is open
// — a scale-down as demand falls, or a manual reset of the desired count.
func TestCircuitBreakerRecoversWhenDesiredDropsWhileOpen(t *testing.T) {
	b := newTestBreaker(2)

	assert.True(t, b.allow(10, 12))
	b.recordScaleUp(10)
	assert.True(t, b.allow(10, 14)) // failure 1
	b.recordScaleUp(10)
	assert.False(t, b.allow(10, 16)) // failure 2 -> trips, desired 16
	assert.Equal(t, circuitOpen, b.state)

	// Desired is lowered to a value the running count already meets: recover
	// rather than staying stuck open until a restart.
	assert.True(t, b.allow(10, 10))
	assert.Equal(t, circuitClosed, b.state)
	assert.Equal(t, 0, b.consecutiveFailures)
}

func TestCircuitBreakerFailedScaleEventCounter(t *testing.T) {
	b := newTestBreaker(5)

	assert.True(t, b.allow(10, 12))
	b.recordScaleUp(10)

	for i := 0; i < 3; i++ {
		b.allow(10, 12) // running stuck at 10 -> failure
		if i < 2 {
			b.recordScaleUp(10)
		}
	}
	assert.Equal(t, 3, b.consecutiveFailures)
}
