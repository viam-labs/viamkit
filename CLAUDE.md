# viamkit

## What this is

A pure-Go toolkit for building Viam modules. Not itself a Viam module — no binary, no `meta.json`, no registered resources. Library only. Consumed by `Palletizing-Module`, `pack-sequencer`, and `workcell-components` (and intended for future siblings + class projects).

Each package owns one concern. They compose; they don't depend on each other except where noted.

## Packages

All thirteen packages below are shipped. See "Versioning + release flow" for
which release each landed in.

| Pkg | One-liner |
|---|---|
| `geom` | `Pose6D`, `Vec3D` + converters to `spatialmath.Pose` / `r3.Vector`. The JSON-serializable shapes the SDK doesn't ship. |
| `contracts` | Two layers: generic DoCommand wire-format codec helpers (`ToMap` / `FromMap[T]` / `MustToMap`), plus typed wire structs + verb constants for the workcell ecosystem (`packsequencer.go`, `pickstation.go`, `pallet.go`, `colors.go`) so producers and consumers share one definition. |
| `lifecycle` | Two-context pattern: cancellable loop ctx (`Stop`, `EnsureLive`, `Ctx`) + timeout-bounded cleanup ctx (`CleanupCtx`, `CtxOrCleanup`). Drop-in for the `cancelCtx + cancelFunc + cleanupCtx()` quartet most modules end up writing. |
| `statemachine` | Generic FSM over a typed state set. `Run` / `Step` / `Goto` / `Reset`. `WithHandlers(map)` declarative dispatch. `WithErrorState` + `WithOnEntry` / `WithOnExit` / `OnTransition` lifecycle hooks. `TimeInState` / `TimeInCycle` / `TimeSinceState` / `IsDone` accessors. |
| `cycle` | Per-cycle duration tracker + rolling stats (min/max/mean/p50/p95). Pairs with `statemachine`'s OnEntry/OnExit hooks. |
| `kinematics` | Pure motion-planning helpers: `YawFromOrientation`, joint-space pre-rotation (`PreRotatedJoints`, `AlignStartJointsToPlaceYaw`), `LastTrajectoryJoints` / `TrajectoryToJointPath` (typed + gRPC trajectory shapes), `InterpolateJointPath`, `FriendlyPlannerError`. |
| `fakes` | In-process programmable fakes for Go unit tests: `Gripper`, `Arm`, `Vision`, `Switch`, `Resource` (DoCommand-only). Per-method `Fn` overrides, atomic call counters, scriptable responses. |
| `watchdog` | Background-poller-with-cancel pattern. `Check` returns Healthy / Lost / Transient; OnFail fires on Lost; OnTransient logs and continues; ShouldExit for clean termination. |
| `viz` | `commonpb.Transform` builders for WorldStateStore producers (the live 3D scene viewer). `Box`, `Sphere`, `Capsule`, `Point` structs each with `ToTransform()` (each carries an optional `Color`). Pose ↔ proto converters. `Removal(uuid)` for stream removals. `Store` + `NewStoreService` for the in-memory WSS-service backing. `TrajectoryTransforms` / `TrajectoryUUIDs` for plan-preview chains. |
| `viz/axes` | Pre-styled coordinate-axis triad publisher. One call returns three colored capsule Transforms at an origin pose. Useful for arm-base / per-component frame visualization. |
| `worldstate` | `referenceframe.WorldState` composition for motion planning: `NewBoxObstacle` / `NewSphereObstacle` geometry constructors, `HeldObject` for gripper-frame attached objects, `WorldObstacles` for the "all in world frame" common case, `Combined` to merge static + dynamic. |
| `verify` | "Plan but don't execute" wrapper around the motion service's `DoCommand("plan", ...)` path. `MarshalPlanRequest` + `ParsePlanResponse` + `Plan` convenience. `TrajectoryToEEPoses` for FK-based trajectory rendering. Encapsulates SDK quirks (the "plan" vs "DoPlan" key, partial-plan format, multi-shape trajectory keys). |
| `operatorapp` | Serves a module's operator web app from any `fs.FS`. `Handler` / `ListenAndServe` host the static frontend and set the `part-id` / `host` / `api-key-id` / `api-key` cookies the browser SDK reads to authenticate to the cell (non-HttpOnly, so JS can read them). `Credentials` + `CredentialsFromEnv` (reads the `VIAM_ROBOT_PART_ID` / `VIAM_ROBOT_FQDN` / `VIAM_API_KEY_ID` / `VIAM_API_KEY` env vars) with `WithCredentials` to override. Replaces the per-module hand-copied webserver + the `viam module local-app-testing` proxy with one runnable Go entrypoint. Serves the frontend; doesn't build it. |

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
- **v0.9.0** — `viz.Color` + Color fields on Box/Sphere/Capsule. ToTransform() serializes into `Transform.Metadata.color` / `Metadata.opacity` (the Viam 3D scene renderer's color convention). Zero-value Color = unset, renderer default applies.
- **v0.10.0** — `statemachine.Machine.RequestExit(state, reason)` (folds the watchdog-driven `forcedExitState ↔ runLoop` pattern from the palletizer into the FSM itself), `viz.TrajectoryTransforms` + `viz.TrajectoryUUIDs` (trajectory-preview waypoints as Sphere transforms — the load-bearing plan-preview piece flagged in the 2026-05-14 dryrun), `viz/axes` subpackage (one-line X/Y/Z triad publisher), `viz.Store` + `viz.NewStoreService` (ready-to-register `worldstatestore.Service` backed by an in-memory Transform map — drops the ~150 LoC WSS-producer dance to ~20).
- **v0.11.0** — `contracts/packsequencer.go`, `contracts/pickstation.go`, `contracts/pallet.go` typed wire-format structs for the workcell-ecosystem registry modules. Producers (workcell-components, pack-sequencer) and consumers (a palletizer module) import these so JSON tags can't drift — a typo on either end becomes a compile error rather than a silent zero. Reverses the v0.6.0 "no module-specific types" direction in light of the 2026-05-15 dryrun's silent-zero failures (`place_start` vs `place_start_in_world`, `width_mm` vs `box_width_mm`, etc.). Plus a `watchdog` package-doc callback-contract summary so the OnFail-fires-on-Lost contract is discoverable from the package overview, not just the option godocs.
- **v0.12.0** — `contracts` catches up with workcell-components 0.4.0 and pack-sequencer 0.3.0: `Color` field on `SetBoxTransformRequest`, shared `contracts.Color` type (with `GetColorResponse` / `SetColorRequest` aliased to it), typed structs for the new 0.4.0 verbs (`GetPalletHomePoseRequest`, `GetCornerPosesResponse`, `GetStatusResponse`, `GetSummaryResponse`, `GetConveyorDirectionResponse`, `SetPalletAttributesRequest`, `SetPickStationAttributesRequest`), and `SetPersistResponse` for the `{persisted, hint}` annotation. Plus `cycle.Stats.HasSamples()` helper + godoc warning that Last/Min/Max/Mean/P50/P95 are meaningful only when `Count > 0`.
- **v0.13.0** — four helpers from the 2026-05-18 dryrun's findings. `kinematics.PreRotatedJoints(currentJoints, currentEEXY, currentEEYawRad, targetXY, targetYawRad, sign)` derives J0+J5 in joint space so the cartesian planner starts in a feasible region (closes the recurring "long transit deadline-exceeds under orientation-lock" finding). `kinematics.AlignStartJointsToPlaceYaw(savedJoints, savedYawRad, targetYawRad, sign)` is the verify-side equivalent for switch-saved-joints replay. `worldstate.GripperHeldBox(name, linkName, dims)` builds the held-box `*LinkInFrame` with the correct `+H/2` gripper-local offset baked in — dryrun-2 mis-diagnosed the sign three times before landing on `+H/2`. `viz.AttachToGripper(uuid, gripperName, dims, color)` is the same convention for the 3D-scene visualization. Plus `contracts.GetPackOrderResponse` + `contracts.PackOrderPlacement` typed structs so consumers don't silent-zero the `placements` / `pose_in_world` / bare-`width_mm` fields like the dryrun-4 hand-roll did.
- **v0.14.0** — `operatorapp`: serve a module's operator web app from an `fs.FS` (`Handler` / `ListenAndServe`) and inject the machine-credential cookies the browser SDK reads (`part-id` / `host` / `api-key-id` / `api-key`, from the `VIAM_*` env vars by default). Collapses the per-module hand-copied webserver and the `viam module local-app-testing` proxy into one `cmd/cli` entrypoint plus a `static/` embed. Built for the training curriculum's operator-app section so students import one helper instead of copying HTTP/cookie plumbing.
- **v0.15.0** — `contracts` typed *client* helpers for the pack-sequencer verbs (`NextBox`, `ReportPlacement`, `GetBoxDims`, `GetPalletHome`, `GetPackOrder`, `GetProgress`, `ResetCursor`, `SetBoxTransform`, `ClearBoxTransform`, `SkipBox`) behind a thin rdk-free `DoCommander` interface. Each wraps the `DoCommand` + `ToMap`/`FromMap` round-trip so a consumer writes `resp, err := contracts.NextBox(ctx, svc)` instead of hand-rolling the map keys and codec call. Purely additive — the producer and the existing types / verb constants / codec helpers are unchanged. Built for the training curriculum's pack-sequencer section so students call a verb instead of plumbing the wire format.

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
├── go.mod / go.sum
├── Makefile          (build / test / lint / fmt / hooks targets)
├── README.md
├── CLAUDE.md         (this file)
├── LICENSE           (Apache License 2.0)
├── .golangci.yaml    (lint config — golangci-lint v2)
├── .github/workflows/ci.yml   (build + test + lint on push / PR)
├── .githooks/pre-commit       (fast gofmt + vet + build hook)
├── docs/
│   └── ecosystem.md  (module-level architecture diagram)
├── geom/
│   ├── poses.go      (Pose6D, Vec3D + SDK converters)
│   └── poses_test.go
├── contracts/
│   ├── codec.go      (ToMap, FromMap[T], MustToMap — generic helpers)
│   ├── colors.go     (shared Color type)
│   ├── packsequencer.go (verb constants + typed structs)
│   ├── pickstation.go   (verb constants + typed structs)
│   ├── pallet.go        (verb constants + typed structs)
│   ├── codec_test.go
│   └── wire_shape_test.go
├── lifecycle/
│   ├── lifecycle.go
│   └── lifecycle_test.go
├── statemachine/
│   ├── machine.go    (Machine[S], Run/Step/Goto/Reset, RequestExit, time accessors)
│   ├── options.go    (WithHandler(s), WithTerminal, WithErrorState, WithOnEntry/Exit, OnTransition)
│   ├── machine_test.go
│   └── example_test.go  (godoc examples)
├── cycle/
│   ├── cycle.go
│   └── cycle_test.go
├── kinematics/
│   ├── doc.go
│   ├── orientation.go    (YawFromOrientation)
│   ├── joints.go         (PreRotatedJoints, AlignStartJointsToPlaceYaw, SignConvention)
│   ├── trajectory.go     (LastTrajectoryJoints, TrajectoryToJointPath, InterpolateJointPath)
│   ├── planner_errors.go (FriendlyPlannerError)
│   ├── joints_test.go
│   └── kinematics_test.go
├── fakes/
│   ├── doc.go
│   ├── gripper.go
│   ├── arm.go
│   ├── vision.go
│   ├── switch.go
│   ├── resource.go      (DoCommand-only stub for pack-sequencer / pick-station-style consumers)
│   └── fakes_test.go
├── watchdog/
│   ├── watchdog.go
│   └── watchdog_test.go
├── viz/
│   ├── doc.go
│   ├── transform.go     (Box / Sphere / Capsule / Point + ToTransform)
│   ├── pose.go          (PoseToProto / PoseFromProto)
│   ├── attach.go        (AttachToGripper)
│   ├── plan.go          (TrajectoryTransforms / TrajectoryUUIDs)
│   ├── store.go         (Store + NewStoreService)
│   ├── *_test.go
│   └── axes/
│       ├── axes.go      (X/Y/Z coordinate-triad publisher)
│       └── axes_test.go
├── worldstate/
│   ├── doc.go
│   ├── worldstate.go    (NewBoxObstacle, NewSphereObstacle, HeldObject, GripperHeldBox, WorldObstacles, Combined)
│   ├── worldstate_test.go
│   └── held_test.go
├── verify/
│   ├── doc.go
│   ├── plan.go          (MarshalPlanRequest, ParsePlanResponse, Plan)
│   ├── trajectory.go    (TrajectoryToEEPoses)
│   └── verify_test.go
└── operatorapp/
    ├── operatorapp.go   (Handler, ListenAndServe, Credentials, CredentialsFromEnv, WithCredentials)
    └── operatorapp_test.go
```

## What's NOT in viamkit (and won't be)

- **Module-specific business logic.** Pack-order math, palletizing waypoint composition, pickup-station geometry — these belong in the consumer module. viamkit is a toolkit, not a robotics framework.
- **DoCommand verb impls.** The typed structs in `contracts` describe the wire format; the handler logic lives in the module that owns the verb.
- **Anything that requires a viam-server connection.** `fakes` and `contracts.ToMap` work in pure Go; runtime resource resolution is the consumer's concern.

(A `Motion` service fake is wanted but deferred — see the Roadmap below.)

## Roadmap (planned but not yet shipped)

`verify`, `worldstate`, and `viz` were on this list and have since shipped
(v0.7.0–v0.8.0). What remains:

| Pkg | Notes |
|---|---|
| `docommand` | Generic verb-table dispatcher. Tiny (~30 LOC). Lower payoff than expected — most consumers' dispatch tables are already small, so this stays deferred. |
| `fakes.Motion` | A `Motion` service fake. Deferred until a real consumer test needs it — the interface is large and most state-transition unit tests can avoid it. |

A minimal end-to-end reference module (`examples/`) plus a walkthrough
tutorial are also planned, to give class onboarding something smaller than
the palletizer to learn module structure from.

### Generalize the palletization-specific API

viamkit is meant to be domain-agnostic. A May 2026 pass cleaned up
palletizer-flavored wording in comments, but a few methods still bake in
palletizing assumptions and need a proper refactor. The palletizer module
(`Palletizing-Module`) consumes these, so any rename or signature change is a
breaking change for it — this must be done **in lockstep with the palletizer**,
not unilaterally.

- `worldstate.GripperHeldBox` / `worldstate.DefaultHeldBoxZPadMM` — box-only;
  bakes a `+H/2` offset and a fixed Z-pad. Generalize to any held geometry, or
  move to the consumer.
- `viz.AttachToGripper` — box-only, hard-codes `Label: "held-box"`. Same call.
- `kinematics.AlignStartJointsToPlaceYaw` — general logic, but the name embeds
  the palletizing "place" verb. Rename (e.g. `AlignWristToYaw`).
- `contracts/packsequencer.go`, `pickstation.go`, `pallet.go`, `colors.go` —
  the entire workcell wire-contract layer. Decide whether it stays as a
  documented second layer or splits into a separate
  `viam-labs/workcell-contracts` module so viamkit proper is domain-free.

## Development

- `make check` — build + lint + test, the same suite CI runs (`.github/workflows/ci.yml`).
- `make fmt` — apply gofmt + goimports.
- `make hooks` — install the `.githooks/pre-commit` hook (run once after cloning).
- Lint config is `.golangci.yaml` (golangci-lint v2; revive rule set mirrors `viamrobotics/rdk`).
