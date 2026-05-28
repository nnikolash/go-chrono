package chrono

import (
	"container/heap"
	"time"
)

// taskQueue is a min-heap of tasks ordered by Deadline. Tasks with equal
// deadlines are ordered by their insertion sequence (seq), which makes pop
// order a deterministic FIFO for ties — stable across builds, runs and
// platforms. nextSeq is the monotonic source of seq values; it is assigned in
// Push (the single point through which all insertions pass).
type taskQueue struct {
	tasks   []*Task
	nextSeq uint64
}

func newTaskQueue() *taskQueue {
	return &taskQueue{
		tasks: make([]*Task, 0, 100),
	}
}

func (q *taskQueue) PushTask(t *Task) {
	heap.Push(q, t)
}

func (q *taskQueue) PopTask() (_ *Task) {
	t, _ := heap.Pop(q).(*Task)
	return t
}

func (q *taskQueue) PeekTask() (_ *Task) {
	return q.tasks[0]
}

func (q *taskQueue) HasExpiredTasks(now time.Time) bool {
	return len(q.tasks) != 0 && !now.Before(q.tasks[0].Deadline)
}

func (q *taskQueue) HasTasks() bool {
	return len(q.tasks) != 0
}

func (q *taskQueue) RemoveTask(t *Task) {
	if t.IsPending() {
		heap.Remove(q, t.indexInQueue)
	}
}

func (q *taskQueue) Len() int { return len(q.tasks) }

func (q *taskQueue) Less(i, j int) bool {
	ti, tj := q.tasks[i], q.tasks[j]
	if ti.Deadline.Equal(tj.Deadline) {
		// Tie-break on insertion order so equal deadlines resolve FIFO
		// deterministically instead of by heap-layout-dependent order.
		return ti.seq < tj.seq
	}
	return ti.Deadline.Before(tj.Deadline)
}

func (q *taskQueue) Swap(i, j int) {
	q.tasks[i], q.tasks[j] = q.tasks[j], q.tasks[i]
	q.tasks[i].indexInQueue, q.tasks[j].indexInQueue = i, j
}

func (q *taskQueue) Push(v interface{}) {
	task := v.(*Task)
	task.indexInQueue = len(q.tasks)
	// seq is assigned exactly once per insertion. heap reorders elements via
	// Swap (not Push), so sift operations never overwrite it.
	task.seq = q.nextSeq
	q.nextSeq++
	q.tasks = append(q.tasks, task)
}

func (q *taskQueue) Pop() interface{} {
	tasks := q.tasks
	n := len(tasks)

	oldestTask := tasks[n-1]
	oldestTask.indexInQueue = -1

	q.tasks = tasks[0 : n-1]

	return oldestTask
}

type Task struct {
	Deadline     time.Time
	Action       func(t *Task, now time.Time) (followingTask *Task)
	indexInQueue int
	// seq is the insertion-order tie-break key for equal deadlines. It is
	// assigned by taskQueue.Push and is not part of the public API.
	seq uint64
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
