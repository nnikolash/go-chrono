package chrono

import (
	"context"
	"sync"
	"time"
)

// Buffering allows to collect all tasks before given moment and then execute them in order.
// This might be useful to run some prerequisite simulation before running tasks in live.
//
// For example, you have market trade indicators and trade stategy which uses them.
// Indicators generate their output based on the even from market, and strategy uses them to make decisions.
// To work immediatelly on program startup, strategy would need some history of indicators data being generated.
// To do you, you can enable buffering, run all the historical market events, which will result in indicators
// "thinking" its past and so generating historical data. And then start strategy.
//
// WARKING: Timer/Ticker methods like Stop()/Reset() are not supported in buffering mode - for buffered events
// they will stop working after EndTasksBuffering() is called.
func NewClockWithBuffering(c Clock) *ClockWithBuffering {
	return &ClockWithBuffering{
		Clock: c,
	}
}

type ClockWithBuffering struct {
	Clock

	bufferingLock    sync.Mutex
	bufferingEnabled bool
	tasksBuffer      *Simulator
}

func (c *ClockWithBuffering) BeginTasksBuffering(timeStart time.Time) {
	c.bufferingLock.Lock()
	defer c.bufferingLock.Unlock()

	if c.bufferingEnabled {
		panic("buffering is already started")
	}

	c.bufferingEnabled = true
	c.tasksBuffer = NewSimulator(timeStart)
}

func (c *ClockWithBuffering) EndTasksBuffering(ctx context.Context, liveTimeStart func() time.Time) error {
	var liveTasks []*Task

	for {
		if _, err := c.tasksBuffer.ProcessAllUntil(ctx, liveTimeStart()); err != nil {
			return err
		}

		var disabled bool
		disabled, liveTasks = c.tryDisableBuffering(liveTimeStart())

		if disabled {
			break
		}
	}

	for _, task := range liveTasks {
		c.processTaskInLive(task)
	}

	return nil
}

func (c *ClockWithBuffering) processTaskInLive(t *Task) {
	c.AfterFunc(time.Until(t.Deadline), func(now time.Time) {
		resTask := t.Run(now)

		if resTask != nil {
			c.processTaskInLive(resTask)
		}
	})
}

func (c *ClockWithBuffering) tryDisableBuffering(liveTasksStart time.Time) (disabled bool, liveTasks []*Task) {
	c.bufferingLock.Lock()
	defer c.bufferingLock.Unlock()

	if !c.bufferingEnabled {
		panic("buffering is not started")
	}

	if c.tasksBuffer.HasExpiredTasks(liveTasksStart) {
		return false, nil
	}

	liveTasks = c.tasksBuffer.PopAllTasks()
	c.tasksBuffer = nil
	c.bufferingEnabled = false

	return true, liveTasks
}

func (c *ClockWithBuffering) AfterFunc(d time.Duration, f func(now time.Time)) Timer {
	c.bufferingLock.Lock()
	defer c.bufferingLock.Unlock()

	if c.bufferingEnabled {
		return c.tasksBuffer.AfterFunc(d, f)
	}

	return c.Clock.AfterFunc(d, f)
}

func (c *ClockWithBuffering) EveryFunc(d time.Duration, f func(now time.Time) bool) Ticker {
	c.bufferingLock.Lock()
	defer c.bufferingLock.Unlock()

	if c.bufferingEnabled {
		return c.tasksBuffer.EveryFunc(d, f)
	}

	return c.Clock.EveryFunc(d, f)
}
