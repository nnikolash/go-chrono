package chrono_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/nnikolash/go-chrono"
	"github.com/stretchr/testify/require"
)

// These cover the package-level convenience wrappers that delegate to
// DefaultClock (a RealClock). They are thin pass-throughs, so the tests just
// confirm the delegation is wired up and behaves like the real clock.

func TestPackage_NowSinceUntil(t *testing.T) {
	t.Parallel()

	t0 := chrono.Now()
	time.Sleep(20 * time.Millisecond)

	require.GreaterOrEqual(t, chrono.Since(t0), 20*time.Millisecond)
	require.Greater(t, chrono.Until(t0.Add(time.Hour)), 50*time.Minute)
}

func TestPackage_AfterFunc_Fires(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	chrono.AfterFunc(20*time.Millisecond, func(now time.Time) {
		close(done)
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("AfterFunc did not fire")
	}
}

func TestPackage_UntilFunc_Fires(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	chrono.UntilFunc(chrono.Now().Add(20*time.Millisecond), func(now time.Time) {
		close(done)
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("UntilFunc did not fire")
	}
}

func TestPackage_EveryFunc_Fires(t *testing.T) {
	t.Parallel()

	var ticks int32
	done := make(chan struct{})
	ticker := chrono.EveryFunc(20*time.Millisecond, func(now time.Time) bool {
		if atomic.AddInt32(&ticks, 1) >= 3 {
			close(done)
			return false
		}
		return true
	})
	defer ticker.Stop()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("EveryFunc did not tick 3 times")
	}
	require.GreaterOrEqual(t, atomic.LoadInt32(&ticks), int32(3))
}
