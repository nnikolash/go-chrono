package chrono_test

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nnikolash/go-chrono"
	"github.com/stretchr/testify/require"
)

func TestRealClock_NowSinceUntil(t *testing.T) {
	t.Parallel()

	c := chrono.NewRealClock()

	t0 := c.Now()
	time.Sleep(20 * time.Millisecond)

	require.GreaterOrEqual(t, c.Since(t0), 20*time.Millisecond)

	future := t0.Add(time.Hour)
	require.Greater(t, c.Until(future), 50*time.Minute)
}

func TestRealClock_AfterFunc_Fires(t *testing.T) {
	t.Parallel()

	c := chrono.NewRealClock()

	var fired int32
	done := make(chan struct{})

	c.AfterFunc(20*time.Millisecond, func(now time.Time) {
		atomic.StoreInt32(&fired, 1)
		close(done)
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("AfterFunc did not fire")
	}
	require.Equal(t, int32(1), atomic.LoadInt32(&fired))
}

func TestRealClock_AfterFunc_Stop(t *testing.T) {
	t.Parallel()

	c := chrono.NewRealClock()

	var fired int32
	timer := c.AfterFunc(100*time.Millisecond, func(now time.Time) {
		atomic.StoreInt32(&fired, 1)
	})

	stopped := timer.Stop()
	require.True(t, stopped, "Stop() should return true for not-yet-fired timer")

	time.Sleep(200 * time.Millisecond)
	require.Equal(t, int32(0), atomic.LoadInt32(&fired), "callback fired despite Stop()")
}

func TestRealClock_AfterFunc_Reset(t *testing.T) {
	t.Parallel()

	c := chrono.NewRealClock()

	var fired int32
	timer := c.AfterFunc(time.Hour, func(now time.Time) {
		atomic.StoreInt32(&fired, 1)
	})

	wasActive := timer.Reset(20 * time.Millisecond)
	require.True(t, wasActive)

	time.Sleep(100 * time.Millisecond)
	require.Equal(t, int32(1), atomic.LoadInt32(&fired))
}

// Bug B regression: AfterFunc(0) on RealClock must be Stop-able.
// Used to return an expiredTimer whose Stop() always reported false; now
// unified with d>0 path via time.AfterFunc so Stop() can race against
// dispatch and actually cancel if it wins.
func TestRealClock_AfterFuncZero_StopCancels(t *testing.T) {
	t.Parallel()

	// We attempt many times — a single Stop() race against time.AfterFunc(0)
	// dispatch isn't deterministic, but at least one of these should cancel
	// before the callback fires. The pre-fix code could NEVER cancel.
	const attempts = 200

	cancelledAtLeastOnce := false

	for i := 0; i < attempts; i++ {
		c := chrono.NewRealClock()
		var fired int32

		timer := c.AfterFunc(0, func(now time.Time) {
			atomic.StoreInt32(&fired, 1)
		})

		stopped := timer.Stop()
		time.Sleep(5 * time.Millisecond)

		if stopped {
			cancelledAtLeastOnce = true
			require.Equal(t, int32(0), atomic.LoadInt32(&fired),
				"Stop() returned true but callback still fired",
			)
		}
	}

	require.True(t, cancelledAtLeastOnce,
		"Stop() never managed to cancel — timer is effectively non-cancellable",
	)
}

func TestRealClock_AfterFunc_StopAfterFire(t *testing.T) {
	t.Parallel()

	c := chrono.NewRealClock()

	done := make(chan struct{})
	timer := c.AfterFunc(10*time.Millisecond, func(now time.Time) {
		close(done)
	})

	<-done
	time.Sleep(10 * time.Millisecond)

	require.False(t, timer.Stop(), "Stop() on already-fired timer should return false")
}

func TestRealClock_UntilFunc(t *testing.T) {
	t.Parallel()

	c := chrono.NewRealClock()

	done := make(chan struct{})
	c.UntilFunc(c.Now().Add(20*time.Millisecond), func(now time.Time) {
		close(done)
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("UntilFunc did not fire")
	}
}

func TestRealClock_EveryFunc_Fires(t *testing.T) {
	t.Parallel()

	c := chrono.NewRealClock()

	var ticks int32
	c.EveryFunc(20*time.Millisecond, func(now time.Time) bool {
		return atomic.AddInt32(&ticks, 1) < 3
	})

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&ticks) >= 3
	}, time.Second, 10*time.Millisecond)
}

// Bug C regression: external Stop() on the *time.Ticker returned by EveryFunc
// must not leak the internal goroutine. stdlib's Ticker.Stop() does NOT close
// the channel, so the `for range ticker.C` reader hangs forever.
//
// Detection: count goroutines before and after Stop(). A leak shows up as a
// goroutine that never finishes after Stop().
func TestRealClock_EveryFunc_ExternalStop(t *testing.T) {
	// NOTE: not parallel — relies on goroutine count.

	// Settle background goroutines from prior tests.
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	c := chrono.NewRealClock()

	var ticks int32
	ticker := c.EveryFunc(20*time.Millisecond, func(now time.Time) bool {
		atomic.AddInt32(&ticks, 1)
		return true
	})

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&ticks) >= 1
	}, time.Second, 5*time.Millisecond)

	ticker.Stop()

	ticksAtStop := atomic.LoadInt32(&ticks)

	time.Sleep(150 * time.Millisecond)

	ticksAfterStop := atomic.LoadInt32(&ticks)
	require.LessOrEqual(t, ticksAfterStop-ticksAtStop, int32(1),
		"after Stop() callback should not keep firing (got %d extra ticks)",
		ticksAfterStop-ticksAtStop,
	)

	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	after := runtime.NumGoroutine()
	require.LessOrEqual(t, after, baseline+1,
		"goroutine leak: baseline=%d after=%d (EveryFunc's reader didn't exit on Stop)",
		baseline, after,
	)
}

func TestRealClock_EveryFunc_Reset(t *testing.T) {
	t.Parallel()

	c := chrono.NewRealClock()

	var ticks int32
	ticker := c.EveryFunc(time.Hour, func(now time.Time) bool {
		atomic.AddInt32(&ticks, 1)
		return true
	})

	ticker.Reset(20 * time.Millisecond)

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&ticks) >= 2
	}, time.Second, 10*time.Millisecond)

	ticker.Stop()
}
