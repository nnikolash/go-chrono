package chrono_test

import (
	"context"
	"testing"
	"time"

	"github.com/nnikolash/go-chrono"
	"github.com/stretchr/testify/require"
)

func TestSimTicker(t *testing.T) {
	t.Parallel()

	s := chrono.NewSimulator(time.Now())

	var res1, res2, res3 []int

	s.EveryFunc(time.Minute, func(now time.Time) bool {
		res1 = append(res1, 1)
		return len(res1) < 3
	})

	ticker2 := s.EveryFunc(time.Minute, func(now time.Time) bool {
		res2 = append(res2, 1)
		return true
	})

	ticker3 := s.EveryFunc(time.Minute, func(now time.Time) bool {
		res3 = append(res3, 1)
		return true
	})

	s.AfterFunc(2*time.Minute+time.Second, func(now time.Time) {
		require.Equal(t, []int{1, 1}, res1)
		require.Equal(t, []int{1, 1}, res2)
		require.Equal(t, []int{1, 1}, res3)

		ticker2.Stop()
		ticker3.Stop()

		s.AfterFunc(time.Minute-time.Second, func(now time.Time) {
			require.Equal(t, []int{1, 1, 1}, res1)
			require.Equal(t, []int{1, 1}, res2)
			require.Equal(t, []int{1, 1}, res3)

			ticker3.Reset(30 * time.Second)

			s.AfterFunc(31*time.Second, func(now time.Time) {
				require.Equal(t, []int{1, 1, 1}, res1)
				require.Equal(t, []int{1, 1}, res2)
				require.Equal(t, []int{1, 1, 1}, res3)
			})

			s.AfterFunc(31*time.Second+time.Minute, func(now time.Time) {
				require.Equal(t, []int{1, 1, 1}, res1)
				require.Equal(t, []int{1, 1}, res2)
				require.Equal(t, []int{1, 1, 1, 1}, res3)

				ticker3.Stop()

				s.AfterFunc(5*time.Minute, func(now time.Time) {
					require.Equal(t, []int{1, 1, 1}, res1)
					require.Equal(t, []int{1, 1}, res2)
					require.Equal(t, []int{1, 1, 1, 1}, res3)
				})
			})
		})
	})

	require.Equal(t, []int(nil), res1)

	s.ProcessAll(context.Background())

	require.Equal(t, []int{1, 1, 1}, res1)
	require.Equal(t, []int{1, 1}, res2)
	require.Equal(t, []int{1, 1, 1, 1}, res3)
}
