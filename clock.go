package chrono

import (
	"sync"
	"time"
)

type Clock interface {
	Now() time.Time
	Since(t time.Time) time.Duration
	Until(t time.Time) time.Duration
	AfterFunc(d time.Duration, f func(now time.Time)) Timer
	UntilFunc(t time.Time, f func(now time.Time)) Timer
	EveryFunc(d time.Duration, f func(now time.Time) bool) Ticker
}

var DefaultClock = NewRealClock()

func Now() time.Time {
	return DefaultClock.Now()
}

func Since(t time.Time) time.Duration {
	return DefaultClock.Since(t)
}

func Until(t time.Time) time.Duration {
	return DefaultClock.Until(t)
}

func AfterFunc(d time.Duration, f func(now time.Time)) Timer {
	return DefaultClock.AfterFunc(d, f)
}

func UntilFunc(t time.Time, f func(now time.Time)) Timer {
	return DefaultClock.UntilFunc(t, f)
}

func EveryFunc(d time.Duration, f func(now time.Time) bool) Ticker {
	return DefaultClock.EveryFunc(d, f)
}

// NewRealClock implements Clock interface for real clock.
// All of its tasks are executed in the same goroutine as the caller,
// to have similar behavior as the simulator.
func NewRealClock() *RealClock {
	return &RealClock{}
}

type RealClock struct {
	handlersLock sync.Mutex
}

var _ Clock = &RealClock{}

func (c *RealClock) Now() time.Time {
	return time.Now()
}

func (c *RealClock) Since(t time.Time) time.Duration {
	return time.Since(t)
}

func (c *RealClock) Until(t time.Time) time.Duration {
	return time.Until(t)
}

func (c *RealClock) AfterFunc(d time.Duration, f func(now time.Time)) Timer {
	if d == 0 {
		go c.executeTask(f)

		return newExpiredTimer(c, f)
	}

	return time.AfterFunc(d, func() {
		c.executeTask(f)
	})
}

func (c *RealClock) executeTask(t func(now time.Time)) {
	c.handlersLock.Lock()
	defer c.handlersLock.Unlock()

	t(time.Now())
}

func (c *RealClock) UntilFunc(t time.Time, f func(now time.Time)) Timer {
	return c.AfterFunc(time.Until(t), f)
}

func (c *RealClock) EveryFunc(d time.Duration, f func(now time.Time) bool) Ticker {
	ticker := time.NewTicker(d)

	go func() {
		for range ticker.C {
			contin := c.invokeTickHandler(f)
			if !contin {
				ticker.Stop()
				return
			}
		}
	}()

	return ticker
}

func (c *RealClock) invokeTickHandler(f func(now time.Time) bool) bool {
	c.handlersLock.Lock()
	defer c.handlersLock.Unlock()

	return f(time.Now())
}
