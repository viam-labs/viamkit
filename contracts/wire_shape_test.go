package contracts

import (
	"reflect"
	"strings"
	"testing"

	"github.com/viam-labs/viamkit/geom"
)

// These tests assert the wire-key set on each contract type. If a tag
// drifts (someone renames `place_start_in_world` to `place_start`,
// say), the test fails immediately rather than allowing a silent-zero
// regression to ship — the exact failure mode the dryrun caught.

func keysOf(t *testing.T, v any) []string {
	t.Helper()
	m, err := ToMap(v)
	if err != nil {
		t.Fatalf("ToMap: %v", err)
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func wantKeys(t *testing.T, got, want []string, label string) {
	t.Helper()
	gset := map[string]bool{}
	for _, k := range got {
		gset[k] = true
	}
	wset := map[string]bool{}
	for _, k := range want {
		wset[k] = true
	}
	var missing, extra []string
	for _, k := range want {
		if !gset[k] {
			missing = append(missing, k)
		}
	}
	for _, k := range got {
		if !wset[k] {
			extra = append(extra, k)
		}
	}
	if len(missing) > 0 || len(extra) > 0 {
		t.Errorf("%s: missing=%v extra=%v (got=%v want=%v)",
			label, missing, extra, sorted(got), sorted(want))
	}
}

func sorted(s []string) string {
	cp := append([]string(nil), s...)
	// Simple insertion sort; the slices are tiny.
	for i := 1; i < len(cp); i++ {
		for j := i; j > 0 && cp[j-1] > cp[j]; j-- {
			cp[j-1], cp[j] = cp[j], cp[j-1]
		}
	}
	return strings.Join(cp, ",")
}

func TestNextBoxResponseKeys(t *testing.T) {
	want := []string{
		"seq", "col", "row", "layer",
		"pose_in_pallet", "approach_offset_in_pallet",
		"place_end_in_world", "place_start_in_world",
		"box_dimensions_mm", "is_complete",
		"total", "placed", "failed", "skipped", "remaining",
	}
	wantKeys(t, keysOf(t, NextBoxResponse{}), want, "NextBoxResponse")
}

func TestGetBoxDimsResponseKeys(t *testing.T) {
	want := []string{"box_length_mm", "box_width_mm", "box_height_mm"}
	wantKeys(t, keysOf(t, GetBoxDimsResponse{}), want, "GetBoxDimsResponse")
}

func TestSetBoxTransformRequestKeys(t *testing.T) {
	// Pose carries through as flat Pose6D — the wire key is "pose"
	// and its value is a nested Pose6D map.
	got := keysOf(t, SetBoxTransformRequest{Seq: 1, Parent: "world", Pose: geom.Pose6D{X: 1}})
	want := []string{"seq", "parent", "pose"}
	wantKeys(t, got, want, "SetBoxTransformRequest")
}

func TestReportPlacementRequestKeys(t *testing.T) {
	// `error` is omitempty, so the empty case shows just seq + success.
	got := keysOf(t, ReportPlacementRequest{Seq: 1, Success: true})
	want := []string{"seq", "success"}
	wantKeys(t, got, want, "ReportPlacementRequest")
}

func TestGetPickHomePoseRequestNested(t *testing.T) {
	// The verb's nested-args calling convention should produce a flat
	// {"box_height_mm": h} payload that sits under cmd[verb].
	m, err := ToMap(GetPickHomePoseRequest{BoxHeightMM: 60})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := m["box_height_mm"]; !ok || got.(float64) != 60 {
		t.Errorf("got %v, want box_height_mm=60", m)
	}
}

func TestPalletAttributesResponseShape(t *testing.T) {
	// The pose field is a nested Pose6D; the color field is a nested
	// GetColorResponse. Confirm they marshal as nested objects, not
	// flat keys.
	m, err := ToMap(GetPalletAttributesResponse{
		WidthMM: 1219, LengthMM: 1016, ThicknessMM: 152,
		Color: GetColorResponse{R: 198, G: 153, B: 97, Opacity: 1},
		Pose:  geom.Pose6D{X: 1, Y: 2, Z: 3, OZ: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m["pose"].(map[string]interface{}); !ok {
		t.Errorf("pose: expected nested map, got %T", m["pose"])
	}
	if c, ok := m["color"].(map[string]interface{}); !ok || c["r"].(float64) != 198 {
		t.Errorf("color: expected nested map with r=198, got %v", m["color"])
	}
}

func TestWireRoundtripNextBox(t *testing.T) {
	orig := NextBoxResponse{
		Seq:               7,
		PlaceEndInWorld:   geom.Pose6D{X: 100, Y: 200, Z: 300, OZ: -1},
		PlaceStartInWorld: geom.Pose6D{X: 100, Y: 200, Z: 410, OZ: -1},
		BoxDimensionsMM:   BoxDimensions{Width: 100, Length: 200, Height: 80},
		IsComplete:        false,
	}
	m, err := ToMap(orig)
	if err != nil {
		t.Fatal(err)
	}
	back, err := FromMap[NextBoxResponse](m)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, orig) {
		t.Errorf("round-trip drift:\norig: %+v\nback: %+v", orig, back)
	}
}
