package contracts

import (
	"math"
	"testing"

	"github.com/viam-labs/viamkit/geom"
)

const eps = 1e-6

func TestRoundTripGetBoxDims(t *testing.T) {
	orig := GetBoxDimsResponse{BoxLengthMM: 200, BoxWidthMM: 100, BoxHeightMM: 80}
	m, err := ToMap(orig)
	if err != nil {
		t.Fatalf("ToMap: %v", err)
	}
	for _, k := range []string{"box_length_mm", "box_width_mm", "box_height_mm"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing wire key %q", k)
		}
	}
	got, err := FromMap[GetBoxDimsResponse](m)
	if err != nil {
		t.Fatalf("FromMap: %v", err)
	}
	if got != orig {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, orig)
	}
}

func TestNextBoxResponseRoundTrip(t *testing.T) {
	orig := NextBoxResponse{
		Seq:               3,
		Col:               1,
		Row:               1,
		Layer:             0,
		PoseInPallet:      geom.Pose6D{X: 100, Y: 200, Z: 40, OZ: -1, Theta: 180},
		PlaceEndInWorld:   geom.Pose6D{X: -500, Y: -200, Z: 50, OZ: -1, Theta: 0},
		PlaceStartInWorld: geom.Pose6D{X: -500, Y: -200, Z: 150, OZ: -1, Theta: 0},
		BoxDimensionsMM:   BoxDimensions{Width: 100, Length: 200, Height: 80},
		Total:             8,
		Placed:            2,
		Remaining:         6,
	}
	m, err := ToMap(orig)
	if err != nil {
		t.Fatalf("ToMap: %v", err)
	}
	// Sanity: nested Pose6D came through as a nested map.
	endMap, ok := m["place_end_in_world"].(map[string]interface{})
	if !ok {
		t.Fatalf("place_end_in_world not a nested map: %T", m["place_end_in_world"])
	}
	if x, _ := endMap["x"].(float64); math.Abs(x-(-500)) > eps {
		t.Errorf("place_end_in_world.x: got %v, want -500", x)
	}
	// And BoxDimensions uses the bare (no-suffix) wire keys.
	dimsMap, ok := m["box_dimensions_mm"].(map[string]interface{})
	if !ok {
		t.Fatalf("box_dimensions_mm not a nested map: %T", m["box_dimensions_mm"])
	}
	if _, hasMM := dimsMap["width_mm"]; hasMM {
		t.Error("box_dimensions_mm should use bare 'width' not 'width_mm'")
	}
	if _, has := dimsMap["width"]; !has {
		t.Error("box_dimensions_mm missing 'width'")
	}

	got, err := FromMap[NextBoxResponse](m)
	if err != nil {
		t.Fatalf("FromMap: %v", err)
	}
	if got.Seq != orig.Seq || got.BoxDimensionsMM != orig.BoxDimensionsMM {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, orig)
	}
}

func TestReportPlacementArgsRoundTrip(t *testing.T) {
	orig := ReportPlacementArgs{Seq: 7, Success: false, Error: "planner timeout"}
	m, err := ToMap(orig)
	if err != nil {
		t.Fatalf("ToMap: %v", err)
	}
	got, err := FromMap[ReportPlacementArgs](m)
	if err != nil {
		t.Fatalf("FromMap: %v", err)
	}
	if got != orig {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, orig)
	}

	// Success case with empty error omits the field on the wire.
	mClean, _ := ToMap(ReportPlacementArgs{Seq: 1, Success: true})
	if _, has := mClean["error"]; has {
		t.Error("empty Error should be omitted from wire (omitempty)")
	}
}

func TestFromMapWrongShape(t *testing.T) {
	// Wrong type for a field surfaces as a JSON error.
	m := map[string]interface{}{"box_length_mm": "not a number"}
	if _, err := FromMap[GetBoxDimsResponse](m); err == nil {
		t.Error("FromMap should reject string for float64 field")
	}
}
