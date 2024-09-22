package example_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nnikolash/go-chrono"
	"github.com/stretchr/testify/require"
)

func TestExample(t *testing.T) {
	t.Parallel()

	simulator := chrono.NewSimulator(time.Now())
	simStart := time.Now()
	simJobDone := startServiceSession(simulator)
	simulator.ProcessAll(context.Background())
	simJobsCount := waitAllJobDone(simJobDone)
	simRunDuration := time.Since(simStart)

	fmt.Printf("Simulation took %v: jobsDone = %v\n", simRunDuration, simJobsCount)

	realClock := chrono.NewRealClock()
	realStart := time.Now()
	realJobDone := startServiceSession(realClock)
	realJobsCount := waitAllJobDone(realJobDone)
	realRunDuration := time.Since(realStart)
	fmt.Printf("Real run took %v: jobsDone = %v\n", realRunDuration, realJobsCount)

	require.NotZero(t, simJobsCount)
	require.Equal(t, simJobsCount, realJobsCount)
	require.True(t, realRunDuration > 100*simRunDuration)
}

func startServiceSession(c chrono.Clock) chan struct{} {
	jobDone := make(chan struct{}, 100)

	svc := NewService(c, jobDone)
	svc.Start()

	scheduleExternalEvents(c, svc)

	return jobDone
}

func waitAllJobDone(jobDone chan struct{}) int {
	totalJobDone := 0

	for range jobDone {
		totalJobDone++
	}

	return totalJobDone
}

func scheduleExternalEvents(c chrono.Clock, svc *Service) {
	c.AfterFunc(0, func(now time.Time) {
		svc.HandleExternalEvent(ExternalEvent{Type: "testEvent1"})
	})

	c.AfterFunc(250*time.Millisecond, func(now time.Time) {
		svc.HandleExternalEvent(ExternalEvent{Type: "waitAndDoAction"})
	})

	c.AfterFunc(450*time.Millisecond, func(now time.Time) {
		svc.HandleExternalEvent(ExternalEvent{Type: "testEvent"})
	})

	c.AfterFunc(1500*time.Millisecond, func(now time.Time) {
		svc.HandleExternalEvent(ExternalEvent{Type: "shutdown"})
	})

}

func NewService(c chrono.Clock, jobsDone chan struct{}) *Service {
	return &Service{
		c:        c,
		jobsDone: jobsDone,
	}
}

type Service struct {
	c         chrono.Clock
	periodJob chrono.Ticker
	jobsDone  chan struct{}
}

type ExternalEvent struct {
	Type string
}

func (p *Service) Start() {
	p.periodJob = p.c.EveryFunc(200*time.Millisecond, func(now time.Time) bool {
		fmt.Printf("%v: Doing periodic job\n", p.c.Now())
		p.jobsDone <- struct{}{}
		return true
	})
}

func (p *Service) HandleExternalEvent(evt ExternalEvent) {
	fmt.Printf("%v: Handling external event %v\n", p.c.Now(), evt.Type)

	switch evt.Type {
	case "waitAndDoAction":
		p.c.AfterFunc(1*time.Second, func(now time.Time) {
			fmt.Printf("%v: Doing action\n", p.c.Now())
			p.jobsDone <- struct{}{}
		})
	case "shutdown":
		fmt.Printf("%v: Shutting down\n", p.c.Now())
		p.periodJob.Stop()
		close(p.jobsDone)
	}

	fmt.Printf("%v: Done handling external event %v\n", p.c.Now(), evt.Type)
}
