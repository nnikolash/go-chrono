package chrono

import "time"

type Ticker interface {
	Reset(d time.Duration)
	Stop()
}

func newSimTicker(sim *Simulator, startTime time.Time, period time.Duration, action func(now time.Time) bool) (*simTicker, *Task) {
	t := &simTicker{
		sim: sim,
		task: newTask(startTime, func(task *Task, now time.Time) *Task {
			if !action(now) {
				return nil
			}

			task.Deadline = task.Deadline.Add(period)

			return task
		}),
	}

	return t, t.task
}

type simTicker struct {
	sim  *Simulator
	task *Task
}

var _ Ticker = &simTicker{}

func (t *simTicker) Stop() {
	t.sim.removeTask(t.task)
}

func (t *simTicker) Reset(d time.Duration) {
	t.sim.resetTask(t.task, d)
}
