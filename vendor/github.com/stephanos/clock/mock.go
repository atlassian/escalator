package clock

import (
	"sync"
	"time"
)

type mock struct {
	base      time.Time
	setAt     time.Time
	frozen    bool
	timeMutex sync.Mutex

	sleep      time.Duration
	sleepMutex sync.Mutex
}

// NewMock returns a new manipulable Clock.
func NewMock() Mock {
	return &mock{
		base:  time.Now(),
		setAt: time.Now(),
		sleep: -1,
	}
}

func (c *mock) Now() time.Time {
	defer c.timeMutex.Unlock()
	c.timeMutex.Lock()

	if c.frozen {
		return c.setAt
	}

	return c.base.Add(c.elapsed())
}

func (c *mock) Set(t time.Time) Mock {
	defer c.timeMutex.Unlock()
	c.timeMutex.Lock()

	c.base = t
	c.setAt = time.Now()
	return c
}

func (c *mock) Add(d time.Duration) Mock {
	defer c.timeMutex.Unlock()
	c.timeMutex.Lock()

	c.base = c.base.Add(d)
	if c.frozen {
		c.setAt = c.setAt.Add(d)
	} else {
		c.setAt = time.Now()
	}
	return c
}

func (c *mock) Freeze() Mock {
	return c.FreezeAt(c.Now())
}

func (c *mock) IsFrozen() bool {
	defer c.timeMutex.Unlock()
	c.timeMutex.Lock()

	return c.frozen
}

func (c *mock) FreezeAt(t time.Time) Mock {
	defer c.timeMutex.Unlock()
	c.timeMutex.Lock()

	c.frozen = true
	c.setAt = t
	return c
}

func (c *mock) Unfreeze() Mock {
	defer c.timeMutex.Unlock()
	c.timeMutex.Lock()

	c.setAt = c.setAt.Add(c.elapsed())
	c.frozen = false
	return c
}

func (c *mock) SetSleep(d time.Duration) Mock {
	defer c.sleepMutex.Unlock()
	c.sleepMutex.Lock()

	c.sleep = d
	return c
}

func (c *mock) NoSleep() Mock {
	c.SetSleep(0)
	return c
}

func (c *mock) ResetSleep() Mock {
	c.SetSleep(-1)
	return c
}

func (c *mock) Sleep(d time.Duration) {
	c.sleepMutex.Lock()
	override := c.sleep
	c.sleepMutex.Unlock()

	if override == -1 {
		time.Sleep(d)
	} else {
		time.Sleep(override)
	}
}

// elapsed returns the Duration between the date the time was set and now.
func (c *mock) elapsed() time.Duration {
	return time.Now().Sub(c.setAt)
}

func (*mock) Tick(d time.Duration) <-chan time.Time {
	// TODO: make mockable
	return time.Tick(d)
}

func (*mock) Ticker(d time.Duration) *time.Ticker {
	// TODO: make mockable
	return time.NewTicker(d)
}

func (c *mock) After(d time.Duration) <-chan time.Time {
	// TODO: make mockable
	return time.After(d)
}
