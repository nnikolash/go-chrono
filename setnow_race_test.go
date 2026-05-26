package chrono_test

import (
	"sync"
	"testing"
	"time"

	"github.com/nnikolash/go-chrono"
)

// TestSimulator_SetNow_RaceWithNow probes whether SetNow's use of RLock is a
// genuine data race or a benign pattern. Run under `go test -race`.
//
// Semantics check: setNow writes to s.now while SetNow holds an RLock; a
// concurrent Now() reader also holds an RLock — two simultaneous RLock holders
// where one mutates shared state is a race by sync.RWMutex rules.
//
// If the user's hypothesis is correct (single-controller convention), this test
// will still surface a race because we deliberately violate that convention.
// That is the point: the public API does not enforce single-controller use.
func TestSimulator_SetNow_RaceWithNow(t *testing.T) {
	t.Parallel()

	s := chrono.NewSimulator(time.Unix(0, 0))

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = s.Now()
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		t0 := time.Unix(0, 0)
		for i := 0; i < 10000; i++ {
			select {
			case <-stop:
				return
			default:
				s.SetNow(t0.Add(time.Duration(i) * time.Millisecond))
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()
}
