# viamkit

A pure-Go toolkit for building [Viam](https://www.viam.com/) modules. It holds
the shared types, contracts, and control-flow patterns that the modules in the
workcell ecosystem all need — so each one doesn't redefine them locally and
drift apart.

**viamkit is not itself a Viam module.** It ships no binary, registers no
resources, and has no `meta.json`. It's a plain Go library, imported by the
modules that need it (currently `workcell-components`, `pack-sequencer`, and a
palletizing module).

Each package owns one concern. They compose freely and, except where noted,
don't depend on each other.

## Installing

viamkit is a normal tagged Go module. Add it to a consumer's `go.mod`:

```
require github.com/viam-labs/viamkit v0.13.0
```

then run `go mod tidy`. Pin a specific tag — pre-1.0, the API may break
between minor versions (see [Versioning](#versioning)).

## Packages

| Package | Summary |
|---|---|
| `geom` | JSON-serializable `Pose6D` / `Vec3D` plus converters to and from `spatialmath.Pose` / `r3.Vector`. |
| `contracts` | DoCommand wire-format codec helpers, plus typed wire structs and verb constants for the workcell ecosystem. |
| `lifecycle` | The two-context pattern modules need: a cancellable loop context and timeout-bounded cleanup contexts. |
| `statemachine` | Generic finite state machine over a typed state set — handlers, terminals, error routing, lifecycle hooks, time accessors. |
| `cycle` | Per-cycle duration tracker with rolling min/max/mean/p50/p95 statistics. |
| `kinematics` | Stateless motion-planning helpers: orientation math, trajectory-shape handling, joint-space pre-rotation, friendly planner errors. |
| `fakes` | Programmable in-process fakes (`Gripper`, `Arm`, `Vision`, `Switch`, `Resource`) for unit-testing modules without viam-server. |
| `watchdog` | Background-poller-with-cancel pattern: poll a check at an interval, fire a callback on failure, exit cleanly. |
| `viz` | `commonpb.Transform` builders for WorldStateStore producers — the live 3D scene viewer. Includes `viz/axes` for coordinate triads. |
| `worldstate` | `referenceframe.WorldState` composition for motion planning: obstacle constructors, held-object attachment, static + dynamic merge. |
| `verify` | "Plan but don't execute" wrapper around the motion service's `DoCommand("plan", …)` path. |

Every package carries a thorough package-level doc comment — `go doc
github.com/viam-labs/viamkit/<pkg>` (or pkg.go.dev) is the reference for the
full API. The sections below take a closer look at the four foundational
packages.

## A closer look

### `geom`

JSON-serializable pose and vector types. `spatialmath.Pose` is an interface and
`spatialmath.OrientationVectorDegrees` carries no position, so the SDK doesn't
provide a flat `{x, y, z, o_x, o_y, o_z, theta}` struct for use in config
attributes or DoCommand payloads. This package fills that gap:

- `Pose6D` — flat 6-DOF pose. `ToPose()` lifts to `spatialmath.Pose`;
  `PoseFrom(p)` is the inverse. `ToMap()` emits the canonical DoCommand JSON
  shape.
- `Vec3D` — flat 3D point/direction. `Normalized()` returns a unit
  `r3.Vector` (zero-input falls back to `(0, 0, 1)`).

```go
import "github.com/viam-labs/viamkit/geom"

home := geom.Pose6D{X: -717, Y: -187, Z: 710, OZ: -1}
pose := home.ToPose()       // spatialmath.Pose, ready for motion.Move
back := geom.PoseFrom(pose) // spatialmath.Pose → flat struct
```

### `contracts`

Helpers for the Viam DoCommand wire format, in two layers.

**Generic codec helpers** turn typed Go structs into the
`map[string]interface{}` shape DoCommand carries over gRPC, and back — so
producers and consumers work with named fields instead of probing map keys:

- `ToMap(v)` — struct → wire map (via JSON tags).
- `FromMap[T](m)` — wire map → typed struct (generic).
- `MustToMap(v)` — panic-on-error variant.

```go
import "github.com/viam-labs/viamkit/contracts"

// Define a wire type with JSON tags...
type StatusResponse struct {
    Phase   string `json:"phase"`
    Healthy bool   `json:"healthy"`
}

// Producer side: typed struct → wire map.
return contracts.ToMap(StatusResponse{Phase: "running", Healthy: true})

// Consumer side: wire map → typed struct.
respMap, _ := svc.DoCommand(ctx, map[string]any{"get_status": true})
resp, _ := contracts.FromMap[StatusResponse](respMap)
```

**Typed wire structs for the workcell ecosystem** — `packsequencer.go`,
`pickstation.go`, `pallet.go`, and `colors.go` ship the verb constants and
request/response structs that the pack-sequencer, pick-station, and pallet
modules exchange. A producer and its consumers import the *same* definition,
so a renamed JSON field becomes a compile error rather than a silently zeroed
value.

### `statemachine`

A small generic finite state machine for cycle-driven modules. You define a
state set (any comparable type — typically a string-typed enum) and a handler
per state; the package owns the dispatch loop, terminal handling, error
routing, step/goto/reset, and the read accessors operator UIs need.

```go
import "github.com/viam-labs/viamkit/statemachine"

type State string
const (
    StateIdle  State = "IDLE"
    StateBusy  State = "BUSY"
    StateDone  State = "DONE"
    StateError State = "ERROR"
)

m := statemachine.New(StateIdle,
    // Register handlers as a single map literal — reads like a
    // state→handler dispatch table:
    statemachine.WithHandlers(map[State]statemachine.Handler[State]{
        StateIdle: doIdle,
        StateBusy: doBusy,
    }),
    statemachine.WithTerminal(StateDone, StateError),
    statemachine.WithErrorState(StateError),  // handler errors land here

    // Per-state lifecycle hooks for setup and teardown:
    statemachine.WithOnEntry(StateBusy, prepareWork),
    statemachine.WithOnExit(StateBusy, func(ctx context.Context, t time.Duration) error {
        logger.Infow("WORK finished", "duration", t)
        return nil
    }),

    statemachine.OnTransition(func(from, to State) {
        logger.Debugw("transition", "from", from, "to", to)
    }),
)

// Pair with the lifecycle package — Run blocks on the loop ctx and exits
// cleanly when it's cancelled, when a terminal state is reached, or on
// unhandled error.
go m.Run(lifecycle.Ctx())
```

Why use it instead of writing your own loop:

- **Cancellation is handled for you.** Run watches `ctx.Done()` between every
  dispatch and inside handler-returned errors. Stop the lifecycle, the machine
  exits cleanly with current state preserved for resume.
- **Error states route correctly.** A handler that returns `(_, err)` lands in
  your configured error state with the error on `LastError()`. No bespoke
  "current state at time of failure" tracking per module.
- **Per-state lifecycle hooks.** `WithOnEntry` / `WithOnExit` separate "what a
  state does" (handler) from "how it sets up and tears down" (hooks). OnExit
  receives the elapsed `timeInState` — useful for logging cycle durations or
  time-bounded teardown.
- **Step / Goto / Reset are free.** Useful for testing isolated states, for
  operator UIs, and for recovery after error. They refuse to run while Run is
  in flight (`ErrAlreadyRunning`) so you can't dual-dispatch by accident.
- **`RequestExit`** lets a background goroutine (a watchdog, a panic-stop
  button) redirect the loop to a chosen state without violating the "no Goto
  during Run" invariant.
- **Time-tracking accessors built in.** `IsDone()`, `TimeInState()`,
  `TimeInCycle()`, `TimeSinceState(s)` — all safe from any goroutine.
  `TimeInCycle` survives Pause→Resume so you measure one cycle, not one
  Run-invocation.
- **Concurrency-safe accessors.** `Current()`, `Running()`, `LastError()`,
  `IsTerminal()`, `States()` — call from any goroutine.

### `lifecycle`

Every Viam module juggles two kinds of context: a cancellable *loop* context
tied to in-flight work (a state-machine loop, a poller), and short-lived
*cleanup* contexts that must run even after the loop is cancelled — to park an
arm or tell a peer the cycle ended. `lifecycle` packages that shape so you
don't reinvent the `cancelCtx + cancelFunc + cleanupCtx()` trio per module.

It touches three of a module's methods, each wanting a different context:

```go
import "github.com/viam-labs/viamkit/lifecycle"

// Construction — start the loop on the lifecycle's context.
func newCellModule(deps resource.Dependencies) (*cellModule, error) {
    m := &cellModule{life: lifecycle.New()}
    go m.sm.Run(m.life.Ctx())            // cancellable via m.life.Stop()
    return m, nil
}

// Reconfigure — stop the loop, notify a peer, start fresh.
func (m *cellModule) Reconfigure(ctx context.Context, _ resource.Dependencies, _ resource.Config) error {
    m.life.Stop()                        // cancels Ctx(); the loop unwinds

    cctx, cancel := m.life.CleanupCtx()  // independent of the loop ctx, 5s-bounded
    defer cancel()
    m.peer.DoCommand(cctx, resetCmd)     // still lands, even though Ctx() is dead

    go m.sm.Run(m.life.EnsureLive())     // EnsureLive mints a fresh ctx
    return nil
}

// Close — terminate for good, then last-gasp teardown.
func (m *cellModule) Close(ctx context.Context) error {
    m.life.Close()                       // loop ctx cancelled permanently
    cctx, cancel := m.life.CleanupCtx()  // CleanupCtx still works after Close
    defer cancel()
    return m.arm.MoveToJointPositions(cctx, parkPositions, nil)
}
```

The rule of thumb: the loop reads `Ctx()` so `Stop` can cancel it; teardown
work uses `CleanupCtx()` (independent of the loop ctx, timeout-bounded); the
next run starts from `EnsureLive()`. Reaching for `Ctx()` in teardown is the
classic mistake — it's cancelled by the very `Stop`/`Close` that triggered the
teardown, so the cleanup RPC fails instantly. `CtxOrCleanup()` folds the
live-or-cleanup choice into one call — use it in a DoCommand handler that
should be cancellable while the loop runs but must still complete during
teardown.

## Development

Clone the repo and use the Makefile:

| Command | What it does |
|---|---|
| `make check` | Build, lint, and test — the full suite CI runs. |
| `make build` | Compile every package. |
| `make test` | Run the suite with the race detector. |
| `make lint` | Run golangci-lint (config in `.golangci.yaml`). |
| `make fmt` | Apply gofmt + goimports formatting in place. |
| `make hooks` | Install the pre-commit hook (run once after cloning). |

CI (`.github/workflows/ci.yml`) runs build + test + lint on every push and
pull request. The pre-commit hook in `.githooks/` runs a fast gofmt + vet +
build check; enable it with `make hooks`.

## Releasing

1. Commit changes to `main` and confirm `make check` passes.
2. Tag and push: `git tag vX.Y.Z && git push origin vX.Y.Z`.
3. In each consumer module, bump `require github.com/viam-labs/viamkit vX.Y.Z`
   and run `go mod tidy`.
4. Re-publish the consumer to the Viam registry — viamkit is statically linked
   into each module's tarball at build time.

## Versioning

Pre-1.0, `v0.x.y` signals the API may break between minor versions; consumers
pin a specific tag. Once the package set stabilizes, a `v1.0.0` will follow
strict semver. The per-release changelog lives in `CLAUDE.md`.

## Planned

- `docommand` — a generic verb-table dispatcher. Deferred: most consumers'
  dispatch tables are already small enough not to need it.
- `fakes.Motion` — a `Motion` service fake. Deferred until a real consumer
  test needs it.
- A minimal end-to-end example module plus a walkthrough tutorial, to give
  class onboarding something smaller than the palletizer to learn module
  structure from.

## License

Apache License 2.0 — see [LICENSE](LICENSE). Copyright 2026 Viam, Inc.
