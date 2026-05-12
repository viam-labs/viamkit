# viamkit

A small Go toolkit for building Viam modules. Holds the shared types, contracts, and patterns that multiple modules in the workcell ecosystem need — so they don't redefine the same things locally and drift apart.

This is **not a Viam module** — it ships no binary, registers no resources, has no `meta.json`. It's a pure Go library imported by the modules that need it.

## Current packages

### `geom`

JSON-serializable pose and vector types. `spatialmath.Pose` is an interface and `spatialmath.OrientationVectorDegrees` carries no position, so the SDK doesn't provide a flat `{x, y, z, o_x, o_y, o_z, theta}` struct for use in config attributes or DoCommand payloads. This package fills that gap with:

- `Pose6D` — flat 6-DOF pose. `ToPose()` lifts to `spatialmath.Pose`; `PoseFrom(p)` is the inverse. `ToMap()` emits the canonical DoCommand JSON shape.
- `Vec3D` — flat 3D point/direction. `Normalized()` returns a unit `r3.Vector` (zero-input falls back to `(0,0,1)`).

```go
import "github.com/viam-labs/viamkit/geom"

home := geom.Pose6D{X: -717, Y: -187, Z: 710, OZ: -1}
pose := home.ToPose()    // spatialmath.Pose ready for motion.Move
back := geom.PoseFrom(p) // spatialmath.Pose → flat struct
```

### `contracts`

Typed Go structs for cross-module DoCommand verbs, plus codec helpers. DoCommand at the protocol level is `map[string]interface{}` over gRPC, but consumers can build/parse it through structs for compile-time safety.

- `ToMap(v)` — struct → wire map.
- `FromMap[T](m)` — wire map → typed struct (generic).
- `MustToMap(v)` — panic-on-error variant for well-known producer types.
- Verb constants for every pack-sequencer + pick-station command.
- Typed structs for `next_box`, `report_placement` (args + response), `get_box_dims`.

```go
import "github.com/viam-labs/viamkit/contracts"

// Producer (pack-sequencer):
return contracts.ToMap(contracts.GetBoxDimsResponse{
    BoxLengthMM: 200, BoxWidthMM: 100, BoxHeightMM: 80,
})

// Consumer (palletizer):
respMap, _ := svc.DoCommand(ctx, map[string]any{contracts.VerbGetBoxDims: true})
dims, _ := contracts.FromMap[contracts.GetBoxDimsResponse](respMap)
boxL := dims.BoxLengthMM
```

### `statemachine`

A small generic finite state machine for cycle-driven modules. You define a state set (any comparable type — typically a string-typed enum) and a handler per state; the package owns the dispatch loop, terminal handling, error routing, step/goto/reset, and the read accessors operator UIs need.

```go
import "github.com/viam-labs/viamkit/statemachine"

type State string
const (
    StateIdle   State = "IDLE"
    StateBusy   State = "BUSY"
    StateDone   State = "DONE"
    StateError  State = "ERROR"
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

- **Cancellation is handled for you.** Run watches `ctx.Done()` between every dispatch and inside handler-returned errors. Stop the lifecycle, the machine exits cleanly with current state preserved for resume.
- **Error states route correctly.** A handler that returns `(_, err)` lands in your configured error state with the error on `LastError()`. No bespoke "current state at time of failure" tracking per module.
- **Per-state lifecycle hooks.** `WithOnEntry` / `WithOnExit` separate "what a state does" (handler) from "how it sets up and tears down" (hooks). OnExit receives the elapsed `timeInState` — useful for logging cycle durations or time-bounded teardown.
- **Step / Goto / Reset are free.** Useful for testing isolated states, for operator UIs, and for recovery after error. They refuse to run while Run is in flight (`ErrAlreadyRunning`) so you can't dual-dispatch by accident.
- **Time-tracking accessors built in.** `IsDone()`, `TimeInState()`, `TimeInCycle()`, `TimeSinceState(s)` — all safe from any goroutine. `TimeInCycle` survives Pause→Resume so you measure one cycle, not one Run-invocation.
- **Concurrency-safe accessors.** `Current()`, `Running()`, `LastError()`, `IsTerminal()`, `States()` — call from any goroutine.

### `lifecycle`

The two contexts a Viam module typically needs to juggle: a cancellable loop context tied to in-flight work, and short-lived cleanup contexts that must execute even after the loop has been cancelled. The package gives you the shape directly so you don't reinvent `cancelCtx + cancelFunc + cleanupCtx()` per module.

```go
import "github.com/viam-labs/viamkit/lifecycle"

l := lifecycle.New()
defer l.Close()

go runLoop(l.Ctx())     // long-running work
...
l.Stop()                // cancels Ctx — in-flight RPCs return context.Canceled
ctx, cancel := l.CleanupCtx()
defer cancel()
peer.DoCommand(ctx, ...)  // bounded by 5s timeout, runs after Stop
...
l.EnsureLive()          // refresh ctx for next run (idempotent on live ctx)
go runLoop(l.Ctx())
```

`CtxOrCleanup()` is a one-call helper for operations that should use the loop ctx if live but still execute when stopped (e.g. an operator-triggered DoCommand handler).

## Roadmap

Packages slated for extraction from the palletizer / pack-sequencer code. Each is a focused refactor on its own and will land here as it gets carved out.

| Package | Status | What it'll hold |
|---|---|---|
| `statemachine` | **shipped** | Generic finite state machine over a typed state set. Handlers per state, terminals, optional error state routing, step/goto/reset for testing and operator UIs, OnTransition hook for logging and metrics. |
| `lifecycle` | **shipped** | Two-context pattern: a cancellable loop context (Stop/EnsureLive/Ctx) plus on-demand cleanup contexts (CleanupCtx, CtxOrCleanup) bounded by timeout. Drop-in for the `cancelCtx + cancelFunc + cleanupCtx()` triad most modules end up writing. |
| `fakes` | **shipped** | Programmable in-process fakes for unit-testing modules without viam-server: `Gripper` (satisfies gripper.Gripper, overridable per method, atomic call counters), `Resource` (DoCommand-only fake with verb-keyed responses and a call log). |
| `cycle` | **shipped** | Per-cycle duration tracker with rolling N-cycle stats (min/max/mean/p50/p95). Start/End/Cancel for the in-flight cycle; pairs naturally with statemachine's OnEntry/OnExit hooks. |
| `docommand` | planned | Generic verb-table dispatcher. The `cmdHandlers []{key, handler}` pattern from the palletizer. ~30 LOC of helper, big DX win. |
| `kinematics` | planned | Pure helpers: `yawFromOrientation`, joint-delta computation for base/wrist rotation, `pickTrajectory` / `lastTrajectoryJoints` / `trajectoryToJointPath` (both typed and gRPC trajectory shapes), `interpolateJointPath`, `friendlyPlannerError`. |
| `verify` | planned | "Plan but don't execute" wrapper around motion service `DoPlan`, with feasibility reporting and downsampled trajectory return. Useful for any motion-heavy module. |
| `worldstate` | planned | Held-object-attached-to-gripper composition; placed-obstacle list builder + caching pattern. |
| `viz` | planned | `commonpb.Transform` builders for WorldStateStore producers — the live 3D scene side. |
| `watchdog` | planned | Background-goroutine pattern: poll a condition, cancel the main op on failure. |
| `fakes` (expand) | partial | `Gripper` and `Resource` shipped; `Arm`, `Vision`, `Motion`, `Switch` to follow as tests need them. |

## Local development

Until viamkit is published, consumers use a local-path `replace` directive in their `go.mod`:

```
require github.com/viam-labs/viamkit v0.0.0-00010101000000-000000000000
replace github.com/viam-labs/viamkit => /home/shrews/viam/viamkit
```

When publishing:

1. `git init` here, `git remote add origin git@github.com:viam-labs/viamkit.git`, push.
2. Tag a version: `git tag v0.1.0 && git push --tags`.
3. In each consumer, drop the `replace` and pin the tag: `require github.com/viam-labs/viamkit v0.1.0`.
4. `go mod tidy` in each consumer.

After that, `git clone && go build` works in any consumer without manual setup, and CI resolves through `proxy.golang.org` like any other Go dependency.

## Versioning

Pre-1.0: `v0.x.y` signals API may break between minor versions. Once the package set stabilizes, cut a `v1.0.0` and follow strict semver.
