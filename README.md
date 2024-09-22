# go-chrono - Time simulation for Go

## What this library can do?

* Simulate events in their chronological order
* Run your implementation in live or in simulation by simply dropping in clock of choice.

This library is used as base for [go-coro](https://github.com/nnikolash/go-coro).

## Installation

`go get github.com/nnikolash/go-chrono`

## Usage

###### Simulation:

```
// Create simulator
timeOrigin := <simulated period start time>
s := chrono.NewSimulator(timeOrigin)

// Schedule events
s.AfterFunc(10*time.Second, ...)
s.AfterFunc(15*time.Second, ...)
...

// Run processing
s.ProcessAll(context.Background())
```

Real time:

```
realClock := chrono.NewRealClock()

realClock.AfterFunc(...)

var c Clock = realClock // Use interface Clock for switching between real clock and simulator
```

## Examples

You can find examples in folder `examples` or in test files `*_test.go`

## Troubleshooting

#### Simulator hangs

Simulator works in a **single thread**. So any blocking code will hang it - sleeping, awaiting for a mutex, using channel, infinite loop etc. So for any delayed action `AfterFunc()` must be used.
If you don't like your code to look like spagetti of callback handlers and want to write blocking code - try [github.com/nnikolash/go-coro](https://github.com/nnikolash/go-coro).

#### Simulator finishes unexpectedly

First time working with simulator might produce confusing issues. That is because usually when we work with real-time programs we are used to make some assumptions, which in simulated time might not be true.

Most of the time these assumptions are related to the time of execution of some code. For example, next code would work perfectly fine in regular program, but won't work in simulation:

```
var c Clock = ...
var recodedEvents []Event = ...
var firstEventTime = recordedEvents[0].Time

const year = 365 * 24 * time.Hour

// This function schedules to be executed one year from now
c.AfterFunc(year, func(_ time.Time) {
   fmt.Println("One year passed - stopping")
   os.Exit(0)
})

// This scheduled to be executed immediatelly
c.AfterFunc(0, func(_ time.Time) {
   for _, e := range recordedEvents
      e := e
      evtTimeOffset := e.Time.Sub(firstEventTime)

      go c.AfterFunc(evtTimeOffset, func(_ time.Time) {
         processEvent(e)
      })
})

// ProcessAll() called for simulation
```

In real world, main function is called, it runs gorouites for each of events, and each goroutine adds scheduled processing of that event. And then program wait one year from its start to exit. Surelly enough, one year would be enough at least to put all those events for processing. But for a simulation.

In simulated world, time between events passed in instant. So in simulation if there will be no events between first and second event in the example, entire year will pass instantly. And although events are scheduled for processing, in this example it is done through goroutine. And starting and executing a goroutine takes time. So there is a short moment of time where there is no events between initial two events. Thus simulator immediatelly jump to last event and exits program.

That's why **goroutines** and **channels** most of the time **should not be used** with the simulator.
