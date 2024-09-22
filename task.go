package chrono

import (
	"container/heap"
	"time"
)

type taskQueue []*Task

func newTaskQueue() *taskQueue {
	t := make(taskQueue, 0, 100)
	return &t
}

func (q *taskQueue) PushTask(t *Task) {
	heap.Push(q, t)
}

func (q *taskQueue) PopTask() (_ *Task) {
	t, _ := heap.Pop(q).(*Task)
	return t
}

func (q *taskQueue) PeekTask() (_ *Task) {
	return (*q)[0]
}

func (q taskQueue) HasExpiredTasks(now time.Time) bool {
	return len(q) != 0 && !now.Before(q[0].Deadline)
}

func (q taskQueue) HasTasks() bool {
	return len(q) != 0
}

func (q *taskQueue) RemoveTask(t *Task) {
	if t.IsPending() {
		heap.Remove(q, t.indexInQueue)
	}
}

func (q taskQueue) Len() int { return len(q) }

func (q taskQueue) Less(i, j int) bool {
	return q[i].Deadline.Before(q[j].Deadline)
}

func (q taskQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	q[i].indexInQueue, q[j].indexInQueue = i, j
}

func (q *taskQueue) Push(v interface{}) {
	task := v.(*Task)
	task.indexInQueue = len(*q)
	*q = append(*q, task)
}

func (q *taskQueue) Pop() interface{} {
	tasks := *q
	n := len(tasks)

	oldestTask := tasks[n-1]
	oldestTask.indexInQueue = -1

	*q = tasks[0 : n-1]

	return oldestTask
}

type Task struct {
	Deadline     time.Time
	Action       func(t *Task, now time.Time) (followingTask *Task)
	indexInQueue int
}

func newTask(deadline time.Time, run func(t *Task, now time.Time) *Task) *Task {
	return &Task{
		Deadline:     deadline,
		Action:       run,
		indexInQueue: -1,
	}
}

func (t *Task) Run(now time.Time) (followingTask *Task) {
	return t.Action(t, now)
}

func (t Task) IsPending() bool {
	return t.indexInQueue != -1
}
