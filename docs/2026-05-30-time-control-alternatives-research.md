# Time-control in Go: alternatives, prior art, and where go-chrono stands

**Date:** 2026-05-30
**Method:** multi-source web research (20 sources fetched, 92 candidate claims, 25 adversarially
verified by 3-vote panels — 24 confirmed, 1 refuted). Citations inline.
**Question:** Are there better/more-standard libraries or approaches than go-chrono's custom
`Clock` interface + heap-based virtual-time `Simulator`, for a deterministic backtester that
shares strategy code with production?

---

## TL;DR / verdict

**Keep the custom `Clock` interface + heap-based `Simulator`. Its design is the textbook-correct
discrete-event-simulation (DES) pattern, not a flawed reinvention.** Every individual design
decision maps onto established DES theory and the proven Go clock-injection idiom.

The only credible "adopt instead of maintain" alternative is **[coder/quartz](https://github.com/coder/quartz)**
— actively maintained, same dual-impl pattern, *and* it already solves the goroutine-wakeup race
that go-chrono currently handles only by lock discipline. Trade-off: pre-1.0 API, and a single
`Advance` is capped at the next event (go-chrono's `Advance`/`ProcessAll` fast-forwards freely).

Go 1.25's stdlib **`testing/synctest`** is excellent but **test-only** — it cannot be the
production-shared backtester engine. It *can* be used to test the Simulator itself.

---

## How go-chrono maps onto DES theory (all confirmed)

The canonical next-event DES loop (Leemis/Park; CMU; SimPy; Wikipedia) is:

1. Remove the most-imminent event from the event list.
2. Advance the clock to that event's time.
3. Process the event / update state.
4. Schedule any spawned future events. Repeat until termination.

`Simulator.Advance()` / `ProcessAll()` implement exactly this. The mapping, decision by decision:

| go-chrono decision | DES status | Source |
|---|---|---|
| `Advance`/`ProcessAll` = pop-next → jump clock → run → enqueue spawned | canonical next-event algorithm | Leemis ch.5; CMU; SimPy |
| min-heap ordered by `Deadline` | standard Future Event List (FEL) structure | Leemis; CMU; SimPy uses `heapq` |
| clock frozen during each callback; time only jumps between events | standard DES semantics | CMU; Wikipedia |
| **monotonic `seq` as secondary key → FIFO on equal deadlines** | **textbook tie-break pattern** | Python `heapq` docs; SimPy; Java `PriorityBlockingQueue` |
| needed a secondary key because `container/heap` is unstable | confirmed: no stability guarantee | Go `container/heap`; Python `heapq` |
| inject a `Clock` interface (real in prod / sim in test) | idiomatic Go dual-use pattern | benbjohnson/clock; coder/quartz |

### The tie-break fix specifically

go-chrono's `seq` fix is **precisely** the standard pattern:

- **Python `heapq`** docs recommend storing `(priority, entry_count, task)` where "the entry count
  serves as a tie-breaker so that two tasks with the same priority are returned in the order they
  were added." (https://docs.python.org/3/library/heapq.html)
- **SimPy** stores `(t, eid, event)` with a "strictly increasing event ID" so the event scheduled
  first is processed first. (https://simpy.readthedocs.io/en/stable/topical_guides/time_and_scheduling.html)
- **Java** `PriorityBlockingQueue` uses the same `AtomicLong` sequence pattern.
- Binary heaps are **not stable** — Python `heapq` says so explicitly; Go's `container/heap` gives
  no ordering guarantee for elements equal under `Less`. A scheduler **must** add its own secondary
  key. (https://pkg.go.dev/container/heap)

Conclusion: the monotonic-`seq` fix is confirmed correct and standard.

---

## Library landscape (2025–2026)

### coder/quartz — the strongest live alternative

- `NewReal()` passes through to stdlib `time` (production); `NewMock(t)` gives deterministic test
  control — the **same** real+fake dependency-injection pattern as go-chrono.
- `AdvanceNext()` advances to the next timer/ticker event (the DES "advance to next event" move),
  returning the duration advanced. **More constrained** than go-chrono: a single advance is capped
  at the next pending event "and no further", so fast-forwarding across many events needs repeated
  `AdvanceNext()` calls.
- **Explicitly targets the goroutine-wakeup race**: a *trap* mechanism plus `AdvanceWaiter`
  (returned by `Advance()`/`Set()`; `.Wait()` blocks until all triggered events complete),
  prioritizing determinism — "the test should run the same each time… (no races)."
- Actively maintained by Coder (v0.3.1, 2026-04-09; vendored in Grafana Loki). Caveat: **pre-1.0**,
  API may shift. (https://github.com/coder/quartz, https://coder.com/blog/introducing-quartz)

### benbjohnson/clock — historically dominant, now dead

- Same `New()` (prod) / `NewMock()` (test) interface pattern — validates the idiom.
- **Archived / read-only since 2023-05-18; "no longer maintained."** Supports only manual
  `Add`/`Set`, **no** auto-advance-to-next-event — weaker than both quartz and go-chrono. Do not
  adopt. (https://github.com/benbjohnson/clock)

### testing/synctest (Go 1.25 stdlib) — great, but test-only

- Graduated from `GOEXPERIMENT=synctest` (Go 1.24) to GA in Go 1.25.
- `synctest.Test(t, f)` runs `f` in an isolated "bubble" where `time` functions use a fake clock
  that "moves forward instantaneously if all goroutines in the bubble are **durably** blocked."
  I/O does **not** durably block — a real limitation for production-like simulation.
- **Deliberately scoped to testing**: `Test` requires `*testing.T`; the Go team **removed** the
  earlier non-test `Run()` API, stating "All our current intended uses for synctest are in tests."
  The legacy `GOEXPERIMENT` `Run()` is removed in Go 1.26.
- **Cannot** serve as a production-shared backtester engine; does not satisfy the dual-use
  constraint. *Can* be used to test the Simulator kernel.
  (https://go.dev/doc/go1.25, https://pkg.go.dev/testing/synctest, https://github.com/golang/go/issues/73567)

### Dedicated Go DES engines — validate the paradigm, wrong shape for dual-use

- **simgo** — SimPy-modeled; one process per goroutine but only one runs at a time (same
  single-threaded-execution / concurrent-scheduling model as our Simulator). ~35 stars.
  (https://github.com/fschuetz04/simgo)
- **godes** — "Build Discrete Event Simulation Models in Go"; advances virtual time via explicit
  `Advance()` (fast-forward, no wall-clock wait). Updated Feb 2025, ~18 importers, pre-v1.0.
  (https://github.com/agoussia/godes)
- Both are process/goroutine **frameworks you build a model inside** — they do **not** offer a thin
  `Clock` the *same unmodified* strategy code runs against in prod. They confirm DES is an
  established Go paradigm, but don't fit "same code in prod and sim" as cleanly as clock injection.

### Named but unverified in this pass

`jonboulle/clockwork`, `tilt-dev/clock`, `k8s.io/utils/clock`, `facebookgo/clock` were requested
but produced no surviving verified claims here — their auto-advance capabilities and current
maintenance status were **not** independently confirmed. Treat as open.

---

## Honest caveats

- The "this matches go-chrono exactly" mappings are **analyst inferences** layered on primary-sourced
  DES facts. The DES facts are unanimous and primary-sourced; **no source independently audited
  go-chrono's actual code.**
- "synctest can't be used for production simulation" is a strong, well-supported inference from
  package design + explicit Go-team statements — but it's about intent/fit, **not** a hard technical
  impossibility. One third-party blogger speculated about DES use; the durably-blocked/I/O limits
  undercut that.
- One refuted claim (1–2 vote): "DES processes events in strictly nondecreasing time order" was
  rejected as overstated — equal-timestamp events and ordering nuance make "strict" wrong, which is
  *why* the tie-break key matters.

---

## Recommended actions for go-chrono

1. **Keep the Simulator** — design is correct; its free multi-event `Advance` is actually more
   flexible than quartz's capped single-advance.
2. **Borrow quartz's race solution**: trap + `AdvanceWaiter` give a *formal* guarantee that tasks
   scheduled from other goroutines are visible before `Advance()` runs them. go-chrono relies on
   lock discipline today — worth verifying equivalence (ties into the documented Simulator-contract
   recommendation). *(Follow-up investigation pending.)*
3. **Test the Simulator with `testing/synctest`** (Go 1.25) — a hybrid where synctest exercises the
   kernel while the kernel remains the production-shared backtester engine.

## Open questions

- Exact capabilities/maintenance of `clockwork`, `tilt-dev/clock`, `k8s.io/utils/clock` (auto-advance
  vs manual-set?).
- Does quartz's "advance only to next event" impose meaningful awkwardness for a backtester that
  wants to fast-forward across many events at once?
- Can the Simulator adopt synctest internally for its own test suite while staying the prod engine?
- How does go-chrono guarantee cross-goroutine-scheduled tasks are deterministically visible before
  `Advance()` runs them, vs quartz's trap/`AdvanceWaiter`?

## Primary sources

- DES theory: Leemis/Park ch.5 (https://www.dmi.unict.it/messina/didat/DES_17_18/leemis_chapter5.pdf),
  CMU intro (https://www.cs.cmu.edu/~music/cmp/archives/cmsip/readings/intro-discrete-event-sim.html),
  Wikipedia (https://en.wikipedia.org/wiki/Discrete-event_simulation)
- Tie-break: Python `heapq` (https://docs.python.org/3/library/heapq.html), SimPy
  (https://simpy.readthedocs.io/en/stable/topical_guides/time_and_scheduling.html), Go
  `container/heap` (https://pkg.go.dev/container/heap), Lemire on stable priority queues
  (https://lemire.me/blog/2017/03/13/stable-priority-queues/)
- Libraries: coder/quartz (https://github.com/coder/quartz), benbjohnson/clock
  (https://github.com/benbjohnson/clock), simgo (https://github.com/fschuetz04/simgo), godes
  (https://github.com/agoussia/godes)
- synctest: Go 1.25 notes (https://go.dev/doc/go1.25), pkg docs
  (https://pkg.go.dev/testing/synctest), Go blog (https://go.dev/blog/synctest), issue #73567
  (https://github.com/golang/go/issues/73567)
- Clock-injection idiom: Go blog "testing time" (https://go.dev/blog/testing-time),
  jonboulle/clockwork (https://github.com/jonboulle/clockwork), k8s clock
  (https://pkg.go.dev/k8s.io/utils/clock)
