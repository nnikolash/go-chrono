package chrono

import "time"

type Timer interface {
	Reset(d time.Duration) bool
	Stop() bool
}

// The timer which cannot be stopped, can be only reset.
// Used to be returned by AfterFunc and UntilFunc when the deadline durection is zero.
func newExpiredTimer(c Clock, f func(now time.Time)) *expiredTimer {
	return &expiredTimer{
		c: c,
		f: f,
	}
}

type expiredTimer struct {
	c Clock
	f func(now time.Time)
}

func (t *expiredTimer) Reset(d time.Duration) bool {
	t.c.AfterFunc(d, t.f)
	return false
}

func (t *expiredTimer) Stop() bool {
	return false
}

func newSimTimer(sim *Simulator, deadline time.Time, action func(now time.Time)) (*simTimer, *Task) {
	t := &simTimer{
		sim: sim,
		task: newTask(deadline, func(_ *Task, now time.Time) *Task {
			action(now)
			return nil
		}),
	}

	return t, t.task
}

type simTimer struct {
	sim  *Simulator
	task *Task
}

var _ Timer = &simTimer{}

func (t *simTimer) Stop() bool {
	return t.sim.removeTask(t.task)
}

func (t *simTimer) Reset(d time.Duration) bool {
	return t.sim.resetTask(t.task, d)
}
