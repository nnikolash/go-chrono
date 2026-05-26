package chrono_test

import (
	"context"
	"sync"
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
		mu        sync.Mutex
		resAfter  []int
		resEvery  []int
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
