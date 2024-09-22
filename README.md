# go-chrono - Time simulation for Go

## Intention

* Simulate events in their chronological order
* Run your implementation in live or in simulation by simply dropping in clock of choice.

It was created to provide time simulation for [go-coro](github.com/nnikolash/go-coro).

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
