package chrono

import (
	"context"
	"sync"
	"time"
)

// Simulator simulates time. Allows to have a control over what an when is executed.
// Upon start, initial events must be placed into queue using AfterFunc, UntilFunc or EveryFunc methods.
// Then, the time can be advanced using Advance or ProcessAll methods.
// Tasks are ran in their chronological order. They can generate additional tasks.
// Simulation happens in a single thread, but tasks can be scheduled from different threads.
func NewSimulator(now time.Time) *Simulator {
	return NewSimulatorWithOpts(now, nil)
}

// Usage lock is lock used to push and pop tasks to/from the task queue,
// and also to check current time. If nil - it is regulat sync.RWMutex.
// Pass NoLock to disable locking if you are sure that all calls are made from the same goroutine.
// There is no simulator lock for the sake of simplicity.
func NewSimulatorWithOpts(now time.Time, usageLock RWLocker) *Simulator {
	if usageLock == nil {
		usageLock = &sync.RWMutex{}
	}

	return &Simulator{
		now:       now,
		taskQueue: newTaskQueue(),
		usageLock: usageLock,
	}
}

type Simulator struct {
	usageLock RWLocker
	now       time.Time
	taskQueue *taskQueue
}

var _ Clock = &Simulator{}

func (s *Simulator) Now() time.Time {
	s.usageLock.RLock()
	defer s.usageLock.RUnlock()

	return s.now
}

func (s *Simulator) SetNow(now time.Time) (time.Time, time.Duration) {
	s.usageLock.RLock()
	defer s.usageLock.RUnlock()

	return s.setNow(now)
}

func (s *Simulator) setNow(newNow time.Time) (time.Time, time.Duration) {
	leap := newNow.Sub(s.now)

	if leap > 0 {
		s.now = newNow
	}

	return s.now, leap
}

// Sets the current time to the next task deadlin, but does not run the task.
func (s *Simulator) Approach() (newNow time.Time, leap time.Duration, hasTasks bool) {
	s.usageLock.Lock()
	defer s.usageLock.Unlock()

	if !s.taskQueue.HasTasks() {
		return s.now, 0, false
	}

	nextTask := s.taskQueue.PeekTask()
	newNow, leap = s.setNow(nextTask.Deadline)

	return newNow, leap, true
}

// Advances the current time to the next task deadline and runs the task.
func (s *Simulator) Advance() (newNow time.Time, leap time.Duration, hadTasks bool) {
	return s.AdvanceIfBefore(time.Time{})
}

// Advances the current time to the next task deadline and runs the task if it is before the specified time.
// If there are no tasks or its deadline comes not specified time, the current time is NOT changed.
func (s *Simulator) AdvanceIfBefore(before time.Time) (newNow time.Time, leap time.Duration, hadExpiredTasks bool) {
	s.usageLock.Lock()

	if !s.taskQueue.HasTasks() {
		s.usageLock.Unlock()
		return s.now, 0, false
	}

	if !before.IsZero() {
		nextTask := s.taskQueue.PeekTask()
		if !nextTask.Deadline.Before(before) {
			s.usageLock.Unlock()
			return s.now, 0, false
		}
	}

	newNow, leap = s.processNextTask(false)

	return newNow, leap, true
}

// Processes all tasks.
// WARNING: If you have periodic tasks, this method will run until you explicitly stop them.
func (s *Simulator) ProcessAll(ctx context.Context) (int, error) {
	return s.ProcessAllUntil(ctx, time.Time{})
}

// Processes all tasks, which are set to fire before the specified time (not including).
func (s *Simulator) ProcessAllUntil(ctx context.Context, until time.Time) (int, error) {
	tasksProcessed := 0

	for ctx.Err() == nil {
		_, _, hadExpiredTasks := s.AdvanceIfBefore(until)

		if !hadExpiredTasks {
			return tasksProcessed, nil
		}

		tasksProcessed++
	}

	return tasksProcessed, ctx.Err()
}

func (s *Simulator) HasExpiredTasks(before time.Time) bool {
	s.usageLock.RLock()
	defer s.usageLock.RUnlock()

	return s.taskQueue.HasExpiredTasks(before)
}

// Returns all the pending tasks, and clears the task queue.
func (s *Simulator) PopAllTasks() []*Task {
	s.usageLock.Lock()
	defer s.usageLock.Unlock()

	tasks := s.taskQueue
	s.taskQueue = newTaskQueue()

	return []*Task(*tasks)
}

func (s *Simulator) processNextTask(keepLock bool) (time.Time, time.Duration) {
	nextTask := s.taskQueue.PopTask()
	now, leap := s.setNow(nextTask.Deadline)
	s.usageLock.Unlock()

	followingTask := nextTask.Run(now)

	if followingTask != nil {
		s.usageLock.Lock()
		s.taskQueue.PushTask(followingTask)
		if !keepLock {
			s.usageLock.Unlock()
		}
	} else if keepLock {
		s.usageLock.Lock()
	}

	return now, leap
}

func (t *Simulator) removeTask(task *Task) (taskWasActive bool) {
	t.usageLock.Lock()
	defer t.usageLock.Unlock()

	if !task.IsPending() {
		return false
	}

	t.taskQueue.RemoveTask(task)

	return true
}

func (t *Simulator) resetTask(task *Task, d time.Duration) (wasPending bool) {
	t.usageLock.Lock()
	defer t.usageLock.Unlock()

	isPending := task.IsPending()

	if isPending {
		t.taskQueue.RemoveTask(task)
	}

	task.Deadline = t.now.Add(d)
	t.taskQueue.PushTask(task)

	return isPending
}

func (s *Simulator) Since(t time.Time) time.Duration {
	s.usageLock.RLock()
	defer s.usageLock.RUnlock()

	return s.now.Sub(t)
}

func (s *Simulator) Until(t time.Time) time.Duration {
	s.usageLock.RLock()
	defer s.usageLock.RUnlock()

	return t.Sub(s.now)
}

func (s *Simulator) AfterFunc(d time.Duration, f func(now time.Time)) Timer {
	s.usageLock.Lock()
	defer s.usageLock.Unlock()

	timer, timerTask := newSimTimer(s, s.now.Add(d), f)
	s.taskQueue.PushTask(timerTask)

	return timer
}

func (s *Simulator) UntilFunc(t time.Time, f func(now time.Time)) Timer {
	s.usageLock.Lock()
	defer s.usageLock.Unlock()

	timer, fireTask := newSimTimer(s, t, f)

	s.taskQueue.PushTask(fireTask)

	return timer
}

func (s *Simulator) EveryFunc(interval time.Duration, f func(now time.Time) bool) Ticker {
	s.usageLock.Lock()
	defer s.usageLock.Unlock()

	ticker, startTask := newSimTicker(s, s.now.Add(interval), interval, f)

	s.taskQueue.PushTask(startTask)

	return ticker
}
