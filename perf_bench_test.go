package chrono_test

import (
	"context"
	"testing"
	"time"

	"github.com/nnikolash/go-chrono"
)

const benchN = 20000

// All tasks share one deadline — worst case for the tie-break: Less always
// falls through to the seq comparison, and sift-down must order equal-deadline
// elements by seq (FIFO) instead of stopping early (non-FIFO).
func BenchmarkProcessAll_EqualDeadlines(b *testing.B) {
	t0 := time.Unix(0, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := chrono.NewSimulator(t0)
		for j := 0; j < benchN; j++ {
			s.AfterFunc(time.Second, func(now time.Time) {})
		}
		s.ProcessAll(context.Background())
	}
}

// All deadlines distinct — Less evaluates Equal (false) then Before, so this
// measures the cost of the extra Equal call on the common non-tied path.
func BenchmarkProcessAll_DistinctDeadlines(b *testing.B) {
	t0 := time.Unix(0, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := chrono.NewSimulator(t0)
		for j := 0; j < benchN; j++ {
			s.AfterFunc(time.Duration(j+1)*time.Nanosecond, func(now time.Time) {})
		}
		s.ProcessAll(context.Background())
	}
}

// Realistic mix: many tasks clustered onto a limited set of deadlines (e.g.
// many positions opening across a bounded number of ticks).
func BenchmarkProcessAll_ClusteredDeadlines(b *testing.B) {
	const buckets = 200
	t0 := time.Unix(0, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := chrono.NewSimulator(t0)
		for j := 0; j < benchN; j++ {
			d := time.Duration((j%buckets)+1) * time.Millisecond
			s.AfterFunc(d, func(now time.Time) {})
		}
		s.ProcessAll(context.Background())
	}
}

// Reschedule churn: a pool of periodic tasks that keep re-queuing themselves,
// exercising the Push path (where seq is assigned) under steady-state load.
func BenchmarkProcessAll_RescheduleChurn(b *testing.B) {
	const tickers = 2000
	const ticksEach = 20
	t0 := time.Unix(0, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := chrono.NewSimulator(t0)
		for j := 0; j < tickers; j++ {
			count := 0
			s.EveryFunc(time.Millisecond, func(now time.Time) bool {
				count++
				return count < ticksEach
			})
		}
		s.ProcessAll(context.Background())
	}
}
