package chrono_test

import (
	"context"
	"testing"
	"time"

	"github.com/nnikolash/go-chrono"
	"github.com/stretchr/testify/require"
)

func TestNoLock_LockUnlock(t *testing.T) {
	t.Parallel()

	var l chrono.NoLock

	l.Lock()
	l.Unlock()

	l.RLock()
	l.RUnlock()
}

func TestNoLock_MultipleRLocks(t *testing.T) {
	t.Parallel()

	var l chrono.NoLock

	l.RLock()
	l.RLock()
	l.RUnlock()
	l.RUnlock()
}

func TestNoLock_DoubleLockPanics(t *testing.T) {
	t.Parallel()

	var l chrono.NoLock
	l.Lock()
	defer l.Unlock()

	require.PanicsWithValue(t, "already locked", func() {
		l.Lock()
	})
}

func TestNoLock_UnlockWithoutLockPanics(t *testing.T) {
	t.Parallel()

	var l chrono.NoLock
	require.PanicsWithValue(t, "not locked", func() {
		l.Unlock()
	})
}

func TestNoLock_RLockWhileLockedPanics(t *testing.T) {
	t.Parallel()

	var l chrono.NoLock
	l.Lock()
	defer l.Unlock()

	require.PanicsWithValue(t, "already locked", func() {
		l.RLock()
	})
}

func TestNoLock_LockWhileRLockedPanics(t *testing.T) {
	t.Parallel()

	var l chrono.NoLock
	l.RLock()
	defer l.RUnlock()

	require.PanicsWithValue(t, "rlocks not zero", func() {
		l.Lock()
	})
}

func TestNoLock_RUnlockWithoutRLockPanics(t *testing.T) {
	t.Parallel()

	var l chrono.NoLock
	require.PanicsWithValue(t, "not rlocked", func() {
		l.RUnlock()
	})
}

// Integration: Simulator with NoLock should work for single-threaded use.
func TestSimulator_WithNoLock(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(0, 0)
	s := chrono.NewSimulatorWithOpts(t0, &chrono.NoLock{})

	var fired bool
	s.AfterFunc(time.Second, func(now time.Time) {
		fired = true
	})

	n, err := s.ProcessAll(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.True(t, fired)
}
