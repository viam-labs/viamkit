package contracts

import "testing"

// Synthetic test types — no module-specific shapes, just enough to
// exercise the codec end-to-end.

type widget struct {
	Name   string  `json:"name"`
	Weight float64 `json:"weight_kg"`
	InUse  bool    `json:"in_use,omitempty"`
}

type pair struct {
	Left  int `json:"left"`
	Right int `json:"right"`
}

type nested struct {
	Title string `json:"title"`
	Inner pair   `json:"inner"`
}

func TestToMapAndBack(t *testing.T) {
	orig := widget{Name: "alpha", Weight: 2.5, InUse: true}
	m, err := ToMap(orig)
	if err != nil {
		t.Fatalf("ToMap: %v", err)
	}
	for _, k := range []string{"name", "weight_kg", "in_use"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing wire key %q in %v", k, m)
		}
	}
	got, err := FromMap[widget](m)
	if err != nil {
		t.Fatalf("FromMap: %v", err)
	}
	if got != orig {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, orig)
	}
}

func TestOmitemptyDropsZero(t *testing.T) {
	m, err := ToMap(widget{Name: "beta", Weight: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, has := m["in_use"]; has {
		t.Error("zero-value InUse should be omitted from wire (omitempty)")
	}
}

func TestNestedRoundTrip(t *testing.T) {
	orig := nested{Title: "test", Inner: pair{Left: 1, Right: 2}}
	m, err := ToMap(orig)
	if err != nil {
		t.Fatal(err)
	}
	// Sanity: inner came through as a nested map.
	innerMap, ok := m["inner"].(map[string]interface{})
	if !ok {
		t.Fatalf("inner not a nested map: %T", m["inner"])
	}
	if l, _ := innerMap["left"].(float64); l != 1 {
		t.Errorf("inner.left: got %v, want 1", l)
	}

	got, err := FromMap[nested](m)
	if err != nil {
		t.Fatal(err)
	}
	if got != orig {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, orig)
	}
}

func TestMustToMapPanicsOnInvalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustToMap should panic on un-marshalable input")
		}
	}()
	// channels are not JSON-marshalable
	_ = MustToMap(struct {
		Ch chan int `json:"ch"`
	}{Ch: make(chan int)})
}

func TestFromMapWrongShape(t *testing.T) {
	m := map[string]interface{}{"weight_kg": "not a number"}
	if _, err := FromMap[widget](m); err == nil {
		t.Error("FromMap should reject string for float64 field")
	}
}
