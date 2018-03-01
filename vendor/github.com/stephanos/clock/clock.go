package clock

import "time"

// Work mirrors the behaviour of Go's time package.
var Work Clock

func init() {
	Work = New()
}

// Now returns the current local time.
func Now() time.Time {
	return Work.Now()
}

// Sleep pauses the current goroutine for at least the duration d.
// A negative or zero duration causes Sleep to return immediately.
func Sleep(d time.Duration) {
	Work.Sleep(d)
}

// After waits for the duration to elapse and then sends the current time
// on the returned channel. It is equivalent to NewTimer(d).C.
func After(d time.Duration) <-chan time.Time {
	return Work.After(d)
}

// Tick is a convenience wrapper for NewTicker providing access to the
// ticking channel only.
func Tick(d time.Duration) <-chan time.Time {
	return Work.Tick(d)
}

// Ticker returns a new Ticker containing a channel that will send the
// time with a period specified by the duration argument. It adjusts
// the intervals or drops ticks to make up for slow receivers.
// The duration d must be greater than zero; if not, Ticker will panic.
func Ticker(d time.Duration) *time.Ticker {
	return Work.Ticker(d)
}

// Clock provides the functions from the time package.
type Clock interface {

	// Now returns the current local time.
	Now() time.Time

	// Sleep pauses the current goroutine for at least the duration d.
	// A negative or zero duration causes Sleep to return immediately.
	Sleep(d time.Duration)

	// After waits for the duration to elapse and then sends the current time
	// on the returned channel. It is equivalent to NewTimer(d).C.
	After(d time.Duration) <-chan time.Time

	// Tick is a convenience wrapper for NewTicker providing access to the
	// ticking channel only.
	Tick(d time.Duration) <-chan time.Time

	// Ticker returns a new Ticker containing a channel that will send the
	// time with a period specified by the duration argument. It adjusts
	// the intervals or drops ticks to make up for slow receivers.
	// The duration d must be greater than zero; if not, Ticker will panic.
	Ticker(d time.Duration) *time.Ticker

	// TODO: At(t time.Time) <-chan time.Time
}

// Mock represents a manipulable Work. It is concurrent-friendly.
type Mock interface {
	Clock

	// ==== manipulate Now()

	// Set applies the passed-in time to the Clock's time.
	Set(t time.Time) Mock

	// Add changes the Clock's time by the passed-in duration.
	Add(d time.Duration) Mock

	// Freeze stops the clock's time.
	Freeze() Mock

	// Freeze stops the clock's time at the passed-in moment.
	FreezeAt(t time.Time) Mock

	// IsFrozen is whether the clock's time is stopped.
	IsFrozen() bool

	// Unfreeze starts the clock's time again.
	Unfreeze() Mock

	// ==== manipulate Sleep()

	// SetSleep overrides the passed-in argument to the Sleep method.
	SetSleep(d time.Duration) Mock

	// NoSleep disables the Sleep method.
	NoSleep() Mock

	// ResetSleep re-enables the default Sleep behaviour.
	ResetSleep() Mock

	// ==== manipulate After()

	// SetAfter overrides the passed-in argument to the After method.
	// SetAfter(d time.Duration) Mock
}
