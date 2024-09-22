package chrono_test

import (
	"context"
	"testing"
	"time"

	"github.com/nnikolash/go-chrono"
	"github.com/stretchr/testify/require"
)

func TestClockTasksBuffering(t *testing.T) {
	t.Parallel()

	c := chrono.NewClockWithBuffering(chrono.NewRealClock())

	c.BeginTasksBuffering(time.Now().Add(-4 * time.Hour))

	var resAfter []int
	var resEvery []int

	c.AfterFunc(2*time.Hour, func(now time.Time) {
		resAfter = append(resAfter, 3)
	})

	c.AfterFunc(0, func(now time.Time) {
		resAfter = append(resAfter, 1)

		c.AfterFunc(3*time.Hour, func(now time.Time) {
			resAfter = append(resAfter, 4)
		})
	})

	c.AfterFunc(time.Hour, func(now time.Time) {
		resAfter = append(resAfter, 2)
	})

	c.AfterFunc(4*time.Hour+time.Second, func(now time.Time) {
		resAfter = append(resAfter, 5)
	})

	c.AfterFunc(time.Second, func(now time.Time) {
		c.EveryFunc(time.Hour, func(now time.Time) bool {
			resEvery = append(resEvery, 1)
			return len(resEvery) < 4
		})
	})

	require.Equal(t, []int(nil), resAfter)

	c.EndTasksBuffering(context.Background(), time.Now)

	require.Equal(t, []int{1, 2, 3, 4}, resAfter)
	require.Equal(t, []int{1, 1, 1}, resEvery)
	time.Sleep(2 * time.Second)
	require.Equal(t, []int{1, 2, 3, 4, 5}, resAfter)
	require.Equal(t, []int{1, 1, 1, 1}, resEvery)
}
