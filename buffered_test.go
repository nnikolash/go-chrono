package chrono_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nnikolash/go-chrono"
	"github.com/stretchr/testify/require"
)

func TestClockTasksBuffering(t *testing.T) {
	t.Parallel()

	c := chrono.NewClockWithBuffering(chrono.NewRealClock())

	c.BeginTasksBuffering(time.Now().Add(-4 * time.Hour))

	var (
		mu       sync.Mutex
		resAfter []int
		resEvery []int
	)
	snapshot := func(s *[]int) []int {
		mu.Lock()
		defer mu.Unlock()
		if *s == nil {
			return nil
		}
		out := make([]int, len(*s))
		copy(out, *s)
		return out
	}
	append1 := func(s *[]int, v int) {
		mu.Lock()
		defer mu.Unlock()
		*s = append(*s, v)
	}

	c.AfterFunc(2*time.Hour, func(now time.Time) {
		append1(&resAfter, 3)
	})

	c.AfterFunc(0, func(now time.Time) {
		append1(&resAfter, 1)

		c.AfterFunc(3*time.Hour, func(now time.Time) {
			append1(&resAfter, 4)
		})
	})

	c.AfterFunc(time.Hour, func(now time.Time) {
		append1(&resAfter, 2)
	})

	c.AfterFunc(4*time.Hour+time.Second, func(now time.Time) {
		append1(&resAfter, 5)
	})

	c.AfterFunc(time.Second, func(now time.Time) {
		c.EveryFunc(time.Hour, func(now time.Time) bool {
			append1(&resEvery, 1)
			return len(snapshot(&resEvery)) < 4
		})
	})

	require.Equal(t, []int(nil), snapshot(&resAfter))

	require.NoError(t, c.EndTasksBuffering(context.Background(), time.Now))

	require.Equal(t, []int{1, 2, 3, 4}, snapshot(&resAfter))
	require.Equal(t, []int{1, 1, 1}, snapshot(&resEvery))

	require.Eventually(t, func() bool {
		return len(snapshot(&resAfter)) == 5 && len(snapshot(&resEvery)) == 4
	}, 3*time.Second, 20*time.Millisecond)

	require.Equal(t, []int{1, 2, 3, 4, 5}, snapshot(&resAfter))
	require.Equal(t, []int{1, 1, 1, 1}, snapshot(&resEvery))
}

func TestClockBuffering_DoubleBeginPanics(t *testing.T) {
	t.Parallel()

	c := chrono.NewClockWithBuffering(chrono.NewRealClock())
	c.BeginTasksBuffering(time.Now())

	require.PanicsWithValue(t, "buffering is already started", func() {
		c.BeginTasksBuffering(time.Now())
	})
}

func TestClockBuffering_EndContextCancel(t *testing.T) {
	t.Parallel()

	c := chrono.NewClockWithBuffering(chrono.NewRealClock())
	t0 := time.Now().Add(-time.Hour)
	c.BeginTasksBuffering(t0)

	var fired bool
	c.AfterFunc(time.Minute, func(now time.Time) { fired = true })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := c.EndTasksBuffering(ctx, time.Now)
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, fired, "no buffered task should run once the context is cancelled")
}

// A ticker scheduled during buffering whose ticks all land in the live window
// is handed to processTaskInLive, where each live tick returns a following task
// (the next tick) and recurses. This exercises that recursive re-scheduling.
func TestClockBuffering_LiveTickerReschedules(t *testing.T) {
	t.Parallel()

	c := chrono.NewClockWithBuffering(chrono.NewRealClock())
	c.BeginTasksBuffering(time.Now())

	var ticks int32
	c.EveryFunc(20*time.Millisecond, func(now time.Time) bool {
		// Stop after 4 ticks; up to then each tick reschedules itself.
		return atomic.AddInt32(&ticks, 1) < 4
	})

	// liveTimeStart == now, so the first tick (now+20ms) is not expired in the
	// buffer and the ticker transitions straight to live.
	require.NoError(t, c.EndTasksBuffering(context.Background(), time.Now))

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&ticks) >= 4
	}, 3*time.Second, 10*time.Millisecond, "live ticker should reschedule itself to 4 ticks")
}

func TestClockBuffering_EveryFunc_NoBuffering(t *testing.T) {
	t.Parallel()

	// With buffering never started, EveryFunc must delegate to the wrapped clock.
	c := chrono.NewClockWithBuffering(chrono.NewRealClock())

	var ticks int32
	done := make(chan struct{})
	ticker := c.EveryFunc(20*time.Millisecond, func(now time.Time) bool {
		if atomic.AddInt32(&ticks, 1) >= 2 {
			close(done)
			return false
		}
		return true
	})
	defer ticker.Stop()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("EveryFunc did not tick through the wrapped clock")
	}
}
