package chrono_test

import (
	"context"
	"testing"
	"time"

	"github.com/nnikolash/go-chrono"
	"github.com/stretchr/testify/require"
)

// TestSimulator_EqualDeadline_InvariantToHeapOccupancy is the core regression
// test for the cross-build non-determinism. It schedules 10 tasks at the same
// deadline and asserts they always fire in FIFO (insertion) order — regardless
// of how many unrelated background tasks occupy the heap at the same time.
//
// On the unfixed code (Deadline-only Less) the pop order of equal-deadline
// tasks is scrambled AND depends on the surrounding heap contents: with 0
// background tasks it came out [0 9 8 7 6 5 4 3 2 1], with 1 it was
// [0 9 8 7 2 5 4 3 6 1], with 2 it was [0 1 8 7 5 6 4 9 2 3], etc. That is
// exactly the "rebuild with different code -> different background tasks ->
// different tie order" failure trading-go hit. With the seq tie-break the
// order is FIFO and immune to heap occupancy.
func TestSimulator_EqualDeadline_InvariantToHeapOccupancy(t *testing.T) {
	t.Parallel()

	targetOrder := func(background int) []int {
		t0 := time.Unix(1_700_000_000, 0)
		s := chrono.NewSimulator(t0)

		// Unrelated background tasks at a later deadline — present in the heap
		// while the target tasks are sifted and popped.
		for i := 0; i < background; i++ {
			s.AfterFunc(10*time.Second, func(now time.Time) {})
		}

		var order []int
		for i := 0; i < 10; i++ {
			i := i
			s.AfterFunc(time.Second, func(now time.Time) {
				order = append(order, i)
			})
		}

		_, err := s.ProcessAll(context.Background())
		require.NoError(t, err)
		return order
	}

	want := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	for _, background := range []int{0, 1, 2, 3, 5, 8, 13, 20, 50} {
		require.Equal(t, want, targetOrder(background),
			"FIFO order must hold with %d background tasks in the heap", background)
	}
}

// TestSimulator_EqualDeadline_FIFOOrder verifies that tasks sharing an
// identical deadline are fired in the order they were scheduled (FIFO),
// instead of a heap-layout-dependent order.
func TestSimulator_EqualDeadline_FIFOOrder(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(1_700_000_000, 0)
	s := chrono.NewSimulator(t0)

	var order []string
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		name := name
		// Identical duration => identical deadline for every task.
		s.AfterFunc(time.Second, func(now time.Time) {
			order = append(order, name)
		})
	}

	n, err := s.ProcessAll(context.Background())
	require.NoError(t, err)
	require.Equal(t, 5, n)
	require.Equal(t, []string{"a", "b", "c", "d", "e"}, order,
		"equal-deadline tasks must fire in insertion order")
}

// TestSimulator_MixedDeadline_OrderedByDeadlineThenSeq verifies the full
// ordering key: primary by deadline, secondary by insertion sequence.
func TestSimulator_MixedDeadline_OrderedByDeadlineThenSeq(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(1_700_000_000, 0)
	s := chrono.NewSimulator(t0)

	type sched struct {
		name string
		d    time.Duration
	}
	// Insertion order below; b-tasks share 5ms, a-tasks share 10ms.
	schedule := []sched{
		{"a1", 10 * time.Millisecond},
		{"a2", 10 * time.Millisecond},
		{"b1", 5 * time.Millisecond},
		{"a3", 10 * time.Millisecond},
		{"b2", 5 * time.Millisecond},
	}

	var order []string
	for _, sc := range schedule {
		name := sc.name
		s.AfterFunc(sc.d, func(now time.Time) {
			order = append(order, name)
		})
	}

	n, err := s.ProcessAll(context.Background())
	require.NoError(t, err)
	require.Equal(t, 5, n)
	// 5ms group first (b1 before b2), then 10ms group (a1, a2, a3) in
	// insertion order.
	require.Equal(t, []string{"b1", "b2", "a1", "a2", "a3"}, order)
}

// TestSimulator_EqualDeadline_StableAcrossRuns asserts the equal-deadline
// order is identical on repeated independent runs. Because the order derives
// from insertion sequence rather than heap layout, it is also stable across
// builds — run this with `go test -count=2` to confirm.
func TestSimulator_EqualDeadline_StableAcrossRuns(t *testing.T) {
	t.Parallel()

	run := func() []string {
		t0 := time.Unix(1_700_000_000, 0)
		s := chrono.NewSimulator(t0)

		var order []string
		for _, name := range []string{"t0", "t1", "t2", "t3", "t4", "t5", "t6"} {
			name := name
			s.AfterFunc(time.Second, func(now time.Time) {
				order = append(order, name)
			})
		}

		_, err := s.ProcessAll(context.Background())
		require.NoError(t, err)
		return order
	}

	want := []string{"t0", "t1", "t2", "t3", "t4", "t5", "t6"}
	for i := 0; i < 8; i++ {
		require.Equal(t, want, run(), "run %d diverged", i)
	}
}

// TestSimulator_EqualDeadline_RescheduledTaskGoesToBack verifies that when a
// task reschedules itself (followingTask) onto an already-occupied deadline,
// it is appended after tasks already queued for that deadline.
func TestSimulator_EqualDeadline_RescheduledTaskGoesToBack(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(1_700_000_000, 0)
	s := chrono.NewSimulator(t0)

	var order []string

	// Fires at +1s, then reschedules itself to fire again at +2s.
	fired := false
	s.AfterFunc(time.Second, func(now time.Time) {
		order = append(order, "self-1s")
		if !fired {
			fired = true
			s.AfterFunc(time.Second, func(now time.Time) {
				order = append(order, "self-2s")
			})
		}
	})
	// Scheduled now at +2s, before the reschedule above happens.
	s.AfterFunc(2*time.Second, func(now time.Time) {
		order = append(order, "other-2s")
	})

	n, err := s.ProcessAll(context.Background())
	require.NoError(t, err)
	require.Equal(t, 3, n)
	// other-2s was inserted (seq) before self-2s (rescheduled later), so it
	// fires first among the +2s group.
	require.Equal(t, []string{"self-1s", "other-2s", "self-2s"}, order)
}
