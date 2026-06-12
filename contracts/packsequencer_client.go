package contracts

import (
	"context"
	"fmt"
)

// DoCommander is the slice of a Viam resource the typed pack-sequencer
// clients below need: just DoCommand. Any rdk resource — including a
// worldstatestore.Service — satisfies it, so these helpers stay free of
// an rdk dependency and can be unit-tested with a fake.
type DoCommander interface {
	DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
}

// The functions below are the typed client for the viam:pack-sequencer
// service. Each one wraps the DoCommand + codec round-trip for a single
// verb: it builds the request map from the verb constant (plus a typed
// request struct where the verb takes a body), calls DoCommand, and
// decodes the reply into its typed response. A consumer writes
//
//	resp, err := contracts.NextBox(ctx, p.packSequencer)
//
// instead of assembling the map keys and calling FromMap by hand. The
// pack-sequencer producer is unchanged; this is consumer-side sugar over
// the same wire format.

// NextBox asks the pack-sequencer for the next box's placement.
func NextBox(ctx context.Context, svc DoCommander) (NextBoxResponse, error) {
	m, err := svc.DoCommand(ctx, map[string]interface{}{VerbNextBox: true})
	if err != nil {
		return NextBoxResponse{}, fmt.Errorf("next_box: %w", err)
	}
	return FromMap[NextBoxResponse](m)
}

// ReportPlacement tells the pack-sequencer how a placement went, so its
// cursor advances (success) or holds for a retry (failure).
func ReportPlacement(ctx context.Context, svc DoCommander, req ReportPlacementRequest) (ReportPlacementResponse, error) {
	body, err := ToMap(req)
	if err != nil {
		return ReportPlacementResponse{}, fmt.Errorf("report_placement encode: %w", err)
	}
	m, err := svc.DoCommand(ctx, map[string]interface{}{VerbReportPlacement: body})
	if err != nil {
		return ReportPlacementResponse{}, fmt.Errorf("report_placement: %w", err)
	}
	return FromMap[ReportPlacementResponse](m)
}

// GetBoxDims returns the pack's box dimensions — the single source of
// truth a palletizer pulls at construction.
func GetBoxDims(ctx context.Context, svc DoCommander) (GetBoxDimsResponse, error) {
	m, err := svc.DoCommand(ctx, map[string]interface{}{VerbGetBoxDims: true})
	if err != nil {
		return GetBoxDimsResponse{}, fmt.Errorf("get_box_dims: %w", err)
	}
	return FromMap[GetBoxDimsResponse](m)
}

// GetPalletHome returns the pallet-home pose, in pallet-local and world
// frames.
func GetPalletHome(ctx context.Context, svc DoCommander) (GetPalletHomeResponse, error) {
	m, err := svc.DoCommand(ctx, map[string]interface{}{VerbGetPalletHome: true})
	if err != nil {
		return GetPalletHomeResponse{}, fmt.Errorf("get_pallet_home: %w", err)
	}
	return FromMap[GetPalletHomeResponse](m)
}

// GetPackOrder returns the full computed pack order plus pallet info.
func GetPackOrder(ctx context.Context, svc DoCommander) (GetPackOrderResponse, error) {
	m, err := svc.DoCommand(ctx, map[string]interface{}{VerbGetPackOrder: true})
	if err != nil {
		return GetPackOrderResponse{}, fmt.Errorf("get_pack_order: %w", err)
	}
	return FromMap[GetPackOrderResponse](m)
}

// GetProgress returns the placed / failed / skipped sets and counts.
func GetProgress(ctx context.Context, svc DoCommander) (GetProgressResponse, error) {
	m, err := svc.DoCommand(ctx, map[string]interface{}{VerbGetProgress: true})
	if err != nil {
		return GetProgressResponse{}, fmt.Errorf("get_progress: %w", err)
	}
	return FromMap[GetProgressResponse](m)
}

// ResetCursor clears the placement cursor back to the first box. The
// service's reply (reset flag + next seq) isn't typed; callers that need
// it can fall back to a raw DoCommand.
func ResetCursor(ctx context.Context, svc DoCommander) error {
	if _, err := svc.DoCommand(ctx, map[string]interface{}{VerbResetCursor: true}); err != nil {
		return fmt.Errorf("reset_cursor: %w", err)
	}
	return nil
}

// SetBoxTransform adds or moves a named box transform in the 3D scene
// (held box, dropoff preview, …).
func SetBoxTransform(ctx context.Context, svc DoCommander, req SetBoxTransformRequest) (SetBoxTransformResponse, error) {
	body, err := ToMap(req)
	if err != nil {
		return SetBoxTransformResponse{}, fmt.Errorf("set_box_transform encode: %w", err)
	}
	m, err := svc.DoCommand(ctx, map[string]interface{}{VerbSetBoxTransform: body})
	if err != nil {
		return SetBoxTransformResponse{}, fmt.Errorf("set_box_transform: %w", err)
	}
	return FromMap[SetBoxTransformResponse](m)
}

// ClearBoxTransform removes a named box transform from the 3D scene.
func ClearBoxTransform(ctx context.Context, svc DoCommander, req ClearBoxTransformRequest) error {
	body, err := ToMap(req)
	if err != nil {
		return fmt.Errorf("clear_box_transform encode: %w", err)
	}
	if _, err := svc.DoCommand(ctx, map[string]interface{}{VerbClearBoxTransform: body}); err != nil {
		return fmt.Errorf("clear_box_transform: %w", err)
	}
	return nil
}

// SkipBox marks a box as skipped without placing it (an operator action).
func SkipBox(ctx context.Context, svc DoCommander, req SkipBoxRequest) error {
	body, err := ToMap(req)
	if err != nil {
		return fmt.Errorf("skip_box encode: %w", err)
	}
	if _, err := svc.DoCommand(ctx, map[string]interface{}{VerbSkipBox: body}); err != nil {
		return fmt.Errorf("skip_box: %w", err)
	}
	return nil
}
