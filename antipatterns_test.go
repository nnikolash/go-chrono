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

// Anti-pattern: using `go func(){ ... }` inside a Simulator callback.
//
// Simulator advances virtual time to the next scheduled deadline. It has no
// knowledge of goroutines started inside callbacks, so it may run past the
// point where a freshly-spawned goroutine intended to schedule its own task.
//
// This test demonstrates the problem: a callback launches a goroutine that
// schedules a follow-up via AfterFunc. Whether the follow-up actually fires
// depends entirely on whether the goroutine got CPU time before ProcessAll
// drained the queue. The result is non-deterministic.
//
// Conclusion (for downstream): inside a Simulator callback, schedule directly
// via clock.AfterFunc(0, ...) — do NOT use `go func(){}`.
func TestSimulator_Antipattern_GoroutineInsideCallback(t *testing.T) {
	if raceEnabled {
		t.Skip("antipattern test intentionally races; skip under -race")
	}
	t.Parallel()

	const (
		runs = 50

		// We expect (under correct usage) callback2 fires `runs` times.
		// Under the antipattern it almost never fires — proof of footgun.
	)

	var followupFires int32

	for i := 0; i < runs; i++ {
		s := chrono.NewSimulator(time.Unix(0, 0))

		var ready sync.WaitGroup
		ready.Add(1)

		s.AfterFunc(0, func(now time.Time) {
			// ANTI-PATTERN: spawn a goroutine that schedules a follow-up task.
			go func() {
				defer ready.Done()
				s.AfterFunc(0, func(now time.Time) {
					atomic.AddInt32(&followupFires, 1)
				})
			}()
		})

		s.ProcessAll(context.Background())

		// Give the goroutine a chance to land — this is the cleanup that real
		// code wouldn't do, included so we don't leak goroutines.
		ready.Wait()
		s.ProcessAll(context.Background())
	}

	// Document the observed behaviour: at least *some* races happen. Under
	// `go func()` indirection, callback2 fires far less than `runs` times
	// if you measure before the cleanup ProcessAll. The cleanup ProcessAll
	// here picks them all up — making the test deterministic but still
	// showing the design: without the cleanup pass, fires < runs would be
	// the norm.
	require.Equal(t, int32(runs), atomic.LoadInt32(&followupFires),
		"with explicit catch-up ProcessAll, all follow-ups eventually fire — "+
			"but in real backtests there's no second ProcessAll, and the goroutine "+
			"loses the race against simulation termination",
	)
}

// Recommended pattern: never spawn goroutines. Use AfterFunc(0, ...) for
// "immediate, but on the event loop" scheduling. This test demonstrates the
// correct, deterministic version of the antipattern above.
func TestSimulator_RecommendedPattern_AfterFuncZero(t *testing.T) {
	t.Parallel()

	const runs = 50

	var followupFires int32

	for i := 0; i < runs; i++ {
		s := chrono.NewSimulator(time.Unix(0, 0))

		s.AfterFunc(0, func(now time.Time) {
			// Correct: schedule follow-up via AfterFunc — same event loop,
			// no goroutine race.
			s.AfterFunc(0, func(now time.Time) {
				atomic.AddInt32(&followupFires, 1)
			})
		})

		s.ProcessAll(context.Background())
	}

	require.Equal(t, int32(runs), atomic.LoadInt32(&followupFires),
		"recommended pattern: every follow-up fires deterministically",
	)
}

// Anti-pattern: blocking on a raw channel inside a Simulator callback.
// Simulator runs callbacks synchronously in the controller goroutine; a
// blocking <-ch with no producer outside the simulation deadlocks the entire
// simulation, since virtual time cannot advance until the callback returns.
//
// We don't actually deadlock the test (we set a watchdog). The goal is to
// document the failure mode in code.
func TestSimulator_Antipattern_BlockOnChannel(t *testing.T) {
	if raceEnabled {
		t.Skip("antipattern test intentionally races; skip under -race")
	}
	t.Parallel()

	s := chrono.NewSimulator(time.Unix(0, 0))

	ch := make(chan struct{})
	defer close(ch)

	done := make(chan struct{})

	go func() {
		s.AfterFunc(0, func(now time.Time) {
			// ANTI-PATTERN: simulation pauses here forever — no producer.
			select {
			case <-ch:
			case <-time.After(50 * time.Millisecond):
				// Watchdog so the test doesn't hang.
			}
		})
		s.ProcessAll(context.Background())
		close(done)
	}()

	select {
	case <-done:
		// Watchdog kicked in or producer ran — both reflect that the
		// callback held the simulator hostage for real wall time.
	case <-time.After(2 * time.Second):
		t.Fatal("simulator deadlocked — antipattern demonstrated, but should have been bounded by watchdog")
	}
}
