package clock

import "time"

type realClock struct{}

// New returns a new Clock that mirrors the behaviour of the time package.
func New() Clock {
	return &realClock{}
}

func (*realClock) Now() time.Time {
	return time.Now()
}

func (*realClock) Sleep(d time.Duration) {
	time.Sleep(d)
}

func (*realClock) Tick(d time.Duration) <-chan time.Time {
	return time.Tick(d)
}

func (*realClock) Ticker(d time.Duration) *time.Ticker {
	return time.NewTicker(d)
}

func (*realClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}
