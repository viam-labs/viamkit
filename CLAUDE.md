# viamkit

## What this is

A pure-Go toolkit for building Viam modules. Not itself a Viam module — no binary, no `meta.json`, no registered resources. Library only. Consumed by `Palletizing-Module`, `pack-sequencer`, and `workcell-components` (and intended for future siblings + class projects).

Each package owns one concern. They compose; they don't depend on each other except where noted.

## Packages (current as of v0.5.0)

| Pkg | One-liner |
|---|---|
| `geom` | `Pose6D`, `Vec3D` + converters to `spatialmath.Pose` / `r3.Vector`. The JSON-serializable shapes the SDK doesn't ship. |
| `contracts` | Generic codec helpers for the Viam DoCommand wire format: `ToMap` / `FromMap[T]` / `MustToMap`. No module-specific types — each consumer defines its own request/response structs and dispatches through these helpers. |
| `lifecycle` | Two-context pattern: cancellable loop ctx (`Stop`, `EnsureLive`, `Ctx`) + timeout-bounded cleanup ctx (`CleanupCtx`, `CtxOrCleanup`). Drop-in for the `cancelCtx + cancelFunc + cleanupCtx()` quartet most modules end up writing. |
| `statemachine` | Generic FSM over a typed state set. `Run` / `Step` / `Goto` / `Reset`. `WithHandlers(map)` declarative dispatch. `WithErrorState` + `WithOnEntry` / `WithOnExit` / `OnTransition` lifecycle hooks. `TimeInState` / `TimeInCycle` / `TimeSinceState` / `IsDone` accessors. |
| `cycle` | Per-cycle duration tracker + rolling stats (min/max/mean/p50/p95). Pairs with `statemachine`'s OnEntry/OnExit hooks. |
| `kinematics` | Pure motion-planning helpers: `YawFromOrientation`, `LastTrajectoryJoints` / `TrajectoryToJointPath` (typed + gRPC trajectory shapes), `InterpolateJointPath`, `FriendlyPlannerError`. |
| `fakes` | In-process programmable fakes for Go unit tests: `Gripper`, `Arm`, `Vision`, `Switch`, `Resource` (DoCommand-only). Per-method `Fn` overrides, atomic call counters, scriptable responses. |
| `watchdog` | Background-poller-with-cancel pattern. `Check` returns Healthy / Lost / Transient; OnFail fires on Lost; OnTransient logs and continues; ShouldExit for clean termination. |
| `viz` | `commonpb.Transform` builders for WorldStateStore producers (the live 3D scene viewer). `Box`, `Sphere`, `Capsule`, `Point` structs each with `ToTransform()`. Pose ↔ proto converters. `Removal(uuid)` helper for stream removals. |
| `worldstate` | `referenceframe.WorldState` composition for motion planning: `NewBoxObstacle` / `NewSphereObstacle` geometry constructors, `HeldObject` for gripper-frame attached objects, `WorldObstacles` for the "all in world frame" common case, `Combined` to merge static + dynamic. |
| `verify` | "Plan but don't execute" wrapper around the motion service's `DoCommand("plan", ...)` path. `MarshalPlanRequest` + `ParsePlanResponse` + `Plan` convenience. `TrajectoryToEEPoses` for FK-based trajectory rendering. Encapsulates SDK quirks (the "plan" vs "DoPlan" key, partial-plan format, multi-shape trajectory keys). |

## Versioning + release flow

Pre-1.0 (`v0.x.y`) signals API may break between minor versions. Pinned to specific tags by consumers via standard `go mod`.

To cut a release:

1. Commit changes to `main`.
2. Tag: `git tag vX.Y.Z && git push origin vX.Y.Z`.
3. In each consumer, bump `require github.com/viam-labs/viamkit vX.Y.Z` and run `go mod tidy`.
4. Re-publish the consumer module to the Viam registry (`make module.tar.gz && viam module upload ...`) — viamkit gets statically linked.

Version bumps so far:
- **v0.1.0** — `geom`, `contracts`, `lifecycle`, `statemachine`
- **v0.2.0** — `fakes` (Gripper + Resource)
- **v0.3.0** — `cycle`
- **v0.4.0** — `kinematics`
- **v0.5.0** — `watchdog`, expanded `fakes` (Arm, Vision, Switch)
- **v0.6.0** — slimmed `contracts` to just codec helpers; workcell-specific verb constants and response structs moved to consumer modules (palletizer's `wire_types.go`)
- **v0.6.1** — `fakes.Resource` deterministic verb dispatch for multi-key requests
- **v0.7.0** — `viz` (WorldStateStore Transform builders) + `worldstate` (motion-planner WorldState composition)
- **v0.8.0** — `verify` (motion-service plan-only wrapper + trajectory FK helper)

## Design conventions

These are followed across packages and should stay consistent as new packages get added:

- **SDK types as the currency.** Functions consume and return `spatialmath.Pose`, `r3.Vector`, `referenceframe.Input`, `context.Context`, `gripper.HoldingStatus`, etc. — not custom wrappers. The only custom types are JSON-shape ones (`Pose6D`, `Vec3D`) that the SDK doesn't provide, and they have explicit `ToPose()` / `PoseFrom()` converters.
- **Functional options pattern** for constructors with optional config (`WithInterval`, `WithCleanupTimeout`, `WithHandler`, etc.). Required parameters go in the constructor signature; optional ones go through `Option`.
- **No logger dependency** in most packages. When a package needs to surface diagnostic info, it does so via callback options (`OnTransient`, `OnFail`) so consumers wire their own logging.
- **Concurrency-safe by default.** Every exposed type holds a `sync.Mutex` internally and accessors are safe from any goroutine. Long-running operations release the lock before invoking callbacks.
- **`time.Time` / `time.Duration` in nanoseconds.** No custom time types. Display formatting is the consumer's call (`.Seconds()` / `.Milliseconds()`).
- **Errors via `fmt.Errorf("...: %w", err)`.** No custom error types unless they're sentinel values matched via `errors.Is`.

## Layout

```
viamkit/
├── go.mod
├── README.md
├── CLAUDE.md         (this file)
├── geom/
│   ├── poses.go
│   └── poses_test.go
├── contracts/
│   ├── codec.go      (ToMap, FromMap[T], MustToMap)
│   ├── packsequencer.go  (verb constants + typed structs)
│   ├── pickstation.go    (verb constants)
│   └── codec_test.go
├── lifecycle/
│   ├── lifecycle.go
│   └── lifecycle_test.go
├── statemachine/
│   ├── machine.go    (Machine[S], Run/Step/Goto/Reset, time accessors)
│   ├── options.go    (WithHandler(s), WithTerminal, WithErrorState, WithOnEntry/Exit, OnTransition)
│   ├── machine_test.go
│   └── example_test.go  (godoc examples)
├── cycle/
│   ├── cycle.go
│   └── cycle_test.go
├── kinematics/
│   ├── doc.go
│   ├── orientation.go   (YawFromOrientation)
│   ├── trajectory.go    (LastTrajectoryJoints, TrajectoryToJointPath, InterpolateJointPath)
│   ├── planner_errors.go (FriendlyPlannerError)
│   └── kinematics_test.go
├── fakes/
│   ├── doc.go
│   ├── gripper.go
│   ├── arm.go
│   ├── vision.go
│   ├── switch.go
│   ├── resource.go      (DoCommand-only stub for pack-sequencer / pick-station-style consumers)
│   └── fakes_test.go
└── watchdog/
    ├── watchdog.go
    └── watchdog_test.go
```

## What's NOT in viamkit (and won't be)

- **Module-specific business logic.** Pack-order math, palletizing waypoint composition, pickup-station geometry — these belong in the consumer module. viamkit is a toolkit, not a robotics framework.
- **DoCommand verb impls.** The typed structs in `contracts` describe the wire format; the handler logic lives in the module that owns the verb.
- **Anything that requires a viam-server connection.** `fakes` and `contracts.ToMap` work in pure Go; runtime resource resolution is the consumer's concern.
- **A `Motion` service fake.** Deferred until a real consumer test needs it — the interface is huge and most state-transition unit tests can avoid it.

## Roadmap (planned but not yet shipped)

| Pkg | Notes |
|---|---|
| `verify` | "Plan but don't execute" wrapper around motion service `DoPlan`, with feasibility reporting and downsampled trajectory return. Generalizes the palletizer's `doVerifyPallet`/`doVerifyPickStation`. |
| `worldstate` | Held-object-attached-to-gripper composition + placed-obstacle list builder. Generalizes the palletizer's `combinedWorldState`. |
| `viz` | `commonpb.Transform` builders for `WorldStateStore` producers — the live 3D scene side. |
| `docommand` | Generic verb-table dispatcher. Tiny (~30 LOC). Lower payoff than expected — most consumers' dispatch tables are already small. |
| `fakes.Motion` | When a real consumer needs it. |
