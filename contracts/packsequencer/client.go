// Package packsequencer is the typed client for the viam:pack-sequencer
// service's DoCommand verbs. Each function wraps the DoCommand + codec
// round-trip and returns a typed response, so a consumer writes
//
//	resp, err := packsequencer.NextBox(ctx, svc)
//
// instead of assembling the request map and decoding the reply by hand.
//
// The request/response types are aliases of the wire structs in the
// parent contracts package, so the producer (the pack-sequencer module,
// which imports contracts.NextBoxResponse) and this consumer-side client
// share one definition — a renamed field is still a compile error on both
// sides, not a silent zero.
package packsequencer

import (
	"context"
	"fmt"

	"github.com/viam-labs/viamkit/contracts"
)

// DoCommander is the slice of a Viam resource these helpers need: just
// DoCommand. Any rdk resource — including a worldstatestore.Service —
// satisfies it, so the client stays free of an rdk dependency.
type DoCommander interface {
	DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
}

// Wire types re-exported from the parent contracts package, so call sites read
// `packsequencer.NextBoxResponse` etc. Each is an alias, not a copy: these are
// the same structs the pack-sequencer producer emits, so there is exactly one
// definition of the wire format and the two ends cannot drift.

// NextBoxResponse is the pack-sequencer's reply to next_box.
type NextBoxResponse = contracts.NextBoxResponse

// ApproachOffset is the per-placement approach vector carried in a next_box reply.
type ApproachOffset = contracts.ApproachOffset

// BoxDimensions is a box's length/width/height in millimetres.
type BoxDimensions = contracts.BoxDimensions

// ReportPlacementRequest is the argument to report_placement.
type ReportPlacementRequest = contracts.ReportPlacementRequest

// ReportPlacementResponse is the pack-sequencer's reply to report_placement.
type ReportPlacementResponse = contracts.ReportPlacementResponse

// GetBoxDimsResponse is the pack-sequencer's reply to get_box_dims.
type GetBoxDimsResponse = contracts.GetBoxDimsResponse

// GetPalletHomeResponse is the pack-sequencer's reply to get_pallet_home.
type GetPalletHomeResponse = contracts.GetPalletHomeResponse

// GetPackOrderResponse is the pack-sequencer's reply to get_pack_order.
type GetPackOrderResponse = contracts.GetPackOrderResponse

// PackOrderPlacement is one placement entry within a pack order.
type PackOrderPlacement = contracts.PackOrderPlacement

// GetProgressResponse is the pack-sequencer's reply to get_progress.
type GetProgressResponse = contracts.GetProgressResponse

// SetBoxTransformRequest is the argument to set_box_transform.
type SetBoxTransformRequest = contracts.SetBoxTransformRequest

// SetBoxTransformResponse is the pack-sequencer's reply to set_box_transform.
type SetBoxTransformResponse = contracts.SetBoxTransformResponse

// ClearBoxTransformRequest is the argument to clear_box_transform.
type ClearBoxTransformRequest = contracts.ClearBoxTransformRequest

// SkipBoxRequest is the argument to skip_box.
type SkipBoxRequest = contracts.SkipBoxRequest

// NextBox asks the pack-sequencer for the next box's placement.
func NextBox(ctx context.Context, svc DoCommander) (NextBoxResponse, error) {
	m, err := svc.DoCommand(ctx, map[string]interface{}{contracts.VerbNextBox: true})
	if err != nil {
		return NextBoxResponse{}, fmt.Errorf("next_box: %w", err)
	}
	return contracts.FromMap[NextBoxResponse](m)
}

// ReportPlacement tells the pack-sequencer how a placement went, so its
// cursor advances (success) or holds for a retry (failure).
func ReportPlacement(ctx context.Context, svc DoCommander, req ReportPlacementRequest) (ReportPlacementResponse, error) {
	body, err := contracts.ToMap(req)
	if err != nil {
		return ReportPlacementResponse{}, fmt.Errorf("report_placement encode: %w", err)
	}
	m, err := svc.DoCommand(ctx, map[string]interface{}{contracts.VerbReportPlacement: body})
	if err != nil {
		return ReportPlacementResponse{}, fmt.Errorf("report_placement: %w", err)
	}
	return contracts.FromMap[ReportPlacementResponse](m)
}

// GetBoxDims returns the pack's box dimensions — the single source of
// truth a palletizer pulls at construction.
func GetBoxDims(ctx context.Context, svc DoCommander) (GetBoxDimsResponse, error) {
	m, err := svc.DoCommand(ctx, map[string]interface{}{contracts.VerbGetBoxDims: true})
	if err != nil {
		return GetBoxDimsResponse{}, fmt.Errorf("get_box_dims: %w", err)
	}
	return contracts.FromMap[GetBoxDimsResponse](m)
}

// GetPalletHome returns the pallet-home pose, in pallet-local and world
// frames.
func GetPalletHome(ctx context.Context, svc DoCommander) (GetPalletHomeResponse, error) {
	m, err := svc.DoCommand(ctx, map[string]interface{}{contracts.VerbGetPalletHome: true})
	if err != nil {
		return GetPalletHomeResponse{}, fmt.Errorf("get_pallet_home: %w", err)
	}
	return contracts.FromMap[GetPalletHomeResponse](m)
}

// GetPackOrder returns the full computed pack order plus pallet info.
func GetPackOrder(ctx context.Context, svc DoCommander) (GetPackOrderResponse, error) {
	m, err := svc.DoCommand(ctx, map[string]interface{}{contracts.VerbGetPackOrder: true})
	if err != nil {
		return GetPackOrderResponse{}, fmt.Errorf("get_pack_order: %w", err)
	}
	return contracts.FromMap[GetPackOrderResponse](m)
}

// GetProgress returns the placed / failed / skipped sets and counts.
func GetProgress(ctx context.Context, svc DoCommander) (GetProgressResponse, error) {
	m, err := svc.DoCommand(ctx, map[string]interface{}{contracts.VerbGetProgress: true})
	if err != nil {
		return GetProgressResponse{}, fmt.Errorf("get_progress: %w", err)
	}
	return contracts.FromMap[GetProgressResponse](m)
}

// ResetCursor clears the placement cursor back to the first box. The
// service's reply (reset flag + next seq) isn't typed; callers that need
// it can fall back to a raw DoCommand.
func ResetCursor(ctx context.Context, svc DoCommander) error {
	if _, err := svc.DoCommand(ctx, map[string]interface{}{contracts.VerbResetCursor: true}); err != nil {
		return fmt.Errorf("reset_cursor: %w", err)
	}
	return nil
}

// SetBoxTransform adds or moves a named box transform in the 3D scene
// (held box, dropoff preview, …).
func SetBoxTransform(ctx context.Context, svc DoCommander, req SetBoxTransformRequest) (SetBoxTransformResponse, error) {
	body, err := contracts.ToMap(req)
	if err != nil {
		return SetBoxTransformResponse{}, fmt.Errorf("set_box_transform encode: %w", err)
	}
	m, err := svc.DoCommand(ctx, map[string]interface{}{contracts.VerbSetBoxTransform: body})
	if err != nil {
		return SetBoxTransformResponse{}, fmt.Errorf("set_box_transform: %w", err)
	}
	return contracts.FromMap[SetBoxTransformResponse](m)
}

// ClearBoxTransform removes a named box transform from the 3D scene.
func ClearBoxTransform(ctx context.Context, svc DoCommander, req ClearBoxTransformRequest) error {
	body, err := contracts.ToMap(req)
	if err != nil {
		return fmt.Errorf("clear_box_transform encode: %w", err)
	}
	if _, err := svc.DoCommand(ctx, map[string]interface{}{contracts.VerbClearBoxTransform: body}); err != nil {
		return fmt.Errorf("clear_box_transform: %w", err)
	}
	return nil
}

// SkipBox marks a box as skipped without placing it (an operator action).
func SkipBox(ctx context.Context, svc DoCommander, req SkipBoxRequest) error {
	body, err := contracts.ToMap(req)
	if err != nil {
		return fmt.Errorf("skip_box encode: %w", err)
	}
	if _, err := svc.DoCommand(ctx, map[string]interface{}{contracts.VerbSkipBox: body}); err != nil {
		return fmt.Errorf("skip_box: %w", err)
	}
	return nil
}
