package chrono_test

import (
	"context"
	"testing"
	"time"

	"github.com/nnikolash/go-chrono"
	"github.com/stretchr/testify/require"
)

func TestSimulator_SetNow_AdvancesForward(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(1_700_000_000, 0)
	s := chrono.NewSimulator(t0)

	newNow, leap := s.SetNow(t0.Add(time.Hour))
	require.Equal(t, t0.Add(time.Hour), newNow)
	require.Equal(t, time.Hour, leap)
	require.Equal(t, t0.Add(time.Hour), s.Now())
}

func TestSimulator_SetNow_DoesNotMoveBackward(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(1_700_000_000, 0)
	s := chrono.NewSimulator(t0.Add(time.Hour))

	newNow, leap := s.SetNow(t0)

	require.Equal(t, t0.Add(time.Hour), newNow, "Now() should stay at original after backward SetNow")
	require.Equal(t, -time.Hour, leap, "leap reflects requested delta even when ignored")
	require.Equal(t, t0.Add(time.Hour), s.Now())
}

func TestSimulator_Approach_DoesNotRunTask(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(0, 0)
	s := chrono.NewSimulator(t0)

	var fired bool
	s.AfterFunc(10*time.Second, func(now time.Time) {
		fired = true
	})

	newNow, leap, hasTasks := s.Approach()
	require.True(t, hasTasks)
	require.Equal(t, t0.Add(10*time.Second), newNow)
	require.Equal(t, 10*time.Second, leap)
	require.False(t, fired, "Approach must not run the task")

	// Now Advance — the task should fire because deadline == now.
	newNow2, _, hadTasks := s.Advance()
	require.True(t, hadTasks)
	require.Equal(t, t0.Add(10*time.Second), newNow2)
	require.True(t, fired)
}

func TestSimulator_Approach_EmptyQueue(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(0, 0)
	s := chrono.NewSimulator(t0)

	newNow, leap, hasTasks := s.Approach()
	require.False(t, hasTasks)
	require.Equal(t, t0, newNow)
	require.Equal(t, time.Duration(0), leap)
}

func TestSimulator_HasExpiredTasks(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(0, 0)
	s := chrono.NewSimulator(t0)

	require.False(t, s.HasExpiredTasks(t0))

	s.AfterFunc(10*time.Second, func(now time.Time) {})

	require.False(t, s.HasExpiredTasks(t0))
	require.False(t, s.HasExpiredTasks(t0.Add(9*time.Second)))
	require.True(t, s.HasExpiredTasks(t0.Add(10*time.Second)),
		"task deadline at +10s should be 'expired' at exactly +10s (not Before)")
	require.True(t, s.HasExpiredTasks(t0.Add(time.Hour)))
}

func TestSimulator_PopAllTasks(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(0, 0)
	s := chrono.NewSimulator(t0)

	s.AfterFunc(10*time.Second, func(now time.Time) {})
	s.AfterFunc(5*time.Second, func(now time.Time) {})

	tasks := s.PopAllTasks()
	require.Len(t, tasks, 2)

	// Queue should now be empty.
	_, _, hadTasks := s.Advance()
	require.False(t, hadTasks)

	tasksAgain := s.PopAllTasks()
	require.Len(t, tasksAgain, 0)
}

func TestSimulator_AdvanceIfBefore_NoExpiredTasks(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(0, 0)
	s := chrono.NewSimulator(t0)

	var fired bool
	s.AfterFunc(time.Hour, func(now time.Time) {
		fired = true
	})

	newNow, leap, hadTasks := s.AdvanceIfBefore(t0.Add(time.Minute))
	require.False(t, hadTasks)
	require.False(t, fired)
	require.Equal(t, t0, newNow, "time should not move if no task is before threshold")
	require.Equal(t, time.Duration(0), leap)
}

func TestSimulator_AdvanceIfBefore_ExpiredTask(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(0, 0)
	s := chrono.NewSimulator(t0)

	var fired bool
	s.AfterFunc(10*time.Second, func(now time.Time) {
		fired = true
	})

	newNow, leap, hadTasks := s.AdvanceIfBefore(t0.Add(time.Hour))
	require.True(t, hadTasks)
	require.True(t, fired)
	require.Equal(t, t0.Add(10*time.Second), newNow)
	require.Equal(t, 10*time.Second, leap)
}

func TestSimulator_ProcessAllUntil_Cutoff(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(0, 0)
	s := chrono.NewSimulator(t0)

	var firedAt []time.Duration
	for _, d := range []time.Duration{1, 5, 10, 50, 100} {
		d := d * time.Second
		s.AfterFunc(d, func(now time.Time) {
			firedAt = append(firedAt, now.Sub(t0))
		})
	}

	n, err := s.ProcessAllUntil(context.Background(), t0.Add(20*time.Second))
	require.NoError(t, err)
	require.Equal(t, 3, n, "tasks at 1s, 5s, 10s should fire (50s and 100s should not)")
	require.Equal(t, []time.Duration{1 * time.Second, 5 * time.Second, 10 * time.Second}, firedAt)
}

func TestSimulator_ProcessAll_ContextCancel(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(0, 0)
	s := chrono.NewSimulator(t0)

	ctx, cancel := context.WithCancel(context.Background())

	var count int
	s.EveryFunc(time.Second, func(now time.Time) bool {
		count++
		if count == 5 {
			cancel()
		}
		return true
	})

	n, err := s.ProcessAll(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.GreaterOrEqual(t, n, 5)
}

func TestSimTimer_Reset_OnPendingTimer(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(0, 0)
	s := chrono.NewSimulator(t0)

	var firedAt time.Duration = -1
	timer := s.AfterFunc(time.Hour, func(now time.Time) {
		firedAt = now.Sub(t0)
	})

	wasActive := timer.Reset(10 * time.Second)
	require.True(t, wasActive, "Reset on pending timer should report wasActive=true")

	s.ProcessAll(context.Background())
	require.Equal(t, 10*time.Second, firedAt)
}

func TestSimTimer_Reset_OnExpiredTimer(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(0, 0)
	s := chrono.NewSimulator(t0)

	var fires int
	timer := s.AfterFunc(10*time.Second, func(now time.Time) {
		fires++
	})

	s.ProcessAll(context.Background())
	require.Equal(t, 1, fires)

	wasActive := timer.Reset(5 * time.Second)
	require.False(t, wasActive, "Reset on already-fired timer should report wasActive=false")

	s.ProcessAll(context.Background())
	require.Equal(t, 2, fires, "Reset on fired timer should re-schedule it")
}

func TestSimTimer_Stop_OnExpiredTimer(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(0, 0)
	s := chrono.NewSimulator(t0)

	timer := s.AfterFunc(10*time.Second, func(now time.Time) {})
	s.ProcessAll(context.Background())

	require.False(t, timer.Stop(), "Stop on already-fired timer should return false")
}

func TestSimulator_Since(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(1_700_000_000, 0)
	s := chrono.NewSimulator(t0)

	require.Equal(t, time.Hour, s.Since(t0.Add(-time.Hour)), "past instant")
	require.Equal(t, time.Duration(0), s.Since(t0), "current instant")
	require.Equal(t, -time.Hour, s.Since(t0.Add(time.Hour)), "future instant")

	s.SetNow(t0.Add(2 * time.Hour))
	require.Equal(t, 3*time.Hour, s.Since(t0.Add(-time.Hour)), "tracks virtual now after SetNow")
}

func TestSimulator_Until(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(1_700_000_000, 0)
	s := chrono.NewSimulator(t0)

	require.Equal(t, time.Hour, s.Until(t0.Add(time.Hour)), "future instant")
	require.Equal(t, time.Duration(0), s.Until(t0), "current instant")
	require.Equal(t, -time.Hour, s.Until(t0.Add(-time.Hour)), "past instant")
}

func TestSimulator_UntilFunc_FiresAtAbsoluteTime(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(0, 0)
	s := chrono.NewSimulator(t0)

	var firedAt time.Duration = -1
	s.UntilFunc(t0.Add(30*time.Second), func(now time.Time) {
		firedAt = now.Sub(t0)
	})

	newNow, leap, hadTasks := s.Advance()
	require.True(t, hadTasks)
	require.Equal(t, 30*time.Second, firedAt, "callback runs at the absolute deadline")
	require.Equal(t, t0.Add(30*time.Second), newNow)
	require.Equal(t, 30*time.Second, leap)
}

// UntilFunc shares the same heap as AfterFunc, so equal absolute deadlines must
// also resolve FIFO by insertion order.
func TestSimulator_UntilFunc_EqualDeadline_FIFOOrder(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(0, 0)
	s := chrono.NewSimulator(t0)
	deadline := t0.Add(time.Minute)

	var order []string
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		name := name
		s.UntilFunc(deadline, func(now time.Time) {
			order = append(order, name)
		})
	}

	n, err := s.ProcessAll(context.Background())
	require.NoError(t, err)
	require.Equal(t, 5, n)
	require.Equal(t, []string{"a", "b", "c", "d", "e"}, order)
}

func TestSimulator_PreservesTimeLocation(t *testing.T) {
	t.Parallel()

	// Simulator should not mangle the TZ of the origin time.
	// (See PROBLEMS_AND_RECOMMENDATIONS.md Recommendation 3.)
	loc, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)

	origin := time.Date(2025, 1, 1, 12, 0, 0, 0, loc)
	s := chrono.NewSimulator(origin)

	require.Equal(t, loc, s.Now().Location())

	s.AfterFunc(time.Hour, func(now time.Time) {})
	s.Advance()
	require.Equal(t, loc, s.Now().Location(),
		"after advancing, time should keep the same Location as the origin")
}
