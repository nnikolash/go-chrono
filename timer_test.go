package chrono_test

import (
	"context"
	"testing"
	"time"

	"github.com/nnikolash/go-chrono"
	"github.com/stretchr/testify/require"
)

func TestSimTimer(t *testing.T) {
	t.Parallel()

	s := chrono.NewSimulator(time.Now())

	timer1fired := false
	timer2fired := false
	timer3fired := false
	timer4fired := false

	s.AfterFunc(2*time.Minute, func(now time.Time) {
		timer1fired = true
	})

	timer2 := s.AfterFunc(2*time.Minute, func(now time.Time) {
		timer2fired = true
	})

	timer3 := s.AfterFunc(2*time.Minute, func(now time.Time) {
		timer3fired = true
	})
	timer4 := s.AfterFunc(2*time.Minute, func(now time.Time) {
		timer4fired = true
	})

	s.AfterFunc(1*time.Minute, func(now time.Time) {
		timer2.Stop()
		timer3.Stop()
		timer4.Reset(3 * time.Minute)
	})

	s.AfterFunc(3*time.Minute, func(now time.Time) {
		require.True(t, timer1fired)
		require.False(t, timer2fired)
		require.False(t, timer3fired)
		require.False(t, timer4fired)

		timer3.Reset(1 * time.Minute)
	})

	// Verification runs at +5min, strictly after the +4min group (timer4 and
	// the reset timer3) has fired. Equal deadlines now resolve FIFO by
	// insertion order, so an observer scheduled at the same +4min deadline as
	// the reset timers would run before them (it was inserted first, at start)
	// and would observe them not-yet-fired. Scheduling it later keeps the
	// intent — confirm timer3/timer4 eventually fire — without depending on
	// intra-deadline ordering.
	s.AfterFunc(5*time.Minute, func(now time.Time) {
		require.True(t, timer1fired)
		require.False(t, timer2fired)
		require.True(t, timer3fired)
		require.True(t, timer4fired)
	})

	s.ProcessAll(context.Background())

	require.True(t, timer1fired)
	require.False(t, timer2fired)
	require.True(t, timer3fired)
	require.True(t, timer4fired)
}
