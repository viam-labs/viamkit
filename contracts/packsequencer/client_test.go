package packsequencer

import (
	"context"
	"errors"
	"testing"

	"github.com/viam-labs/viamkit/contracts"
)

// fakeDoCommander records the request and returns a canned reply/error.
type fakeDoCommander struct {
	gotCmd map[string]interface{}
	reply  map[string]interface{}
	err    error
}

func (f *fakeDoCommander) DoCommand(_ context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	f.gotCmd = cmd
	if f.err != nil {
		return nil, f.err
	}
	return f.reply, nil
}

func TestNextBox(t *testing.T) {
	f := &fakeDoCommander{reply: map[string]interface{}{
		"seq":         3,
		"is_complete": false,
		"total":       8,
	}}
	resp, err := NextBox(context.Background(), f)
	if err != nil {
		t.Fatalf("NextBox: %v", err)
	}
	if _, ok := f.gotCmd[contracts.VerbNextBox]; !ok {
		t.Fatalf("request missing verb %q: %v", contracts.VerbNextBox, f.gotCmd)
	}
	if resp.Seq != 3 || resp.IsComplete || resp.Total != 8 {
		t.Errorf("decoded wrong: %+v", resp)
	}
}

func TestReportPlacement_NestsRequestUnderVerb(t *testing.T) {
	f := &fakeDoCommander{reply: map[string]interface{}{"acknowledged": true, "next_seq": 4}}
	resp, err := ReportPlacement(context.Background(), f, ReportPlacementRequest{Seq: 3, Success: true})
	if err != nil {
		t.Fatalf("ReportPlacement: %v", err)
	}
	body, ok := f.gotCmd[contracts.VerbReportPlacement].(map[string]interface{})
	if !ok {
		t.Fatalf("request body not nested under %q: %v", contracts.VerbReportPlacement, f.gotCmd)
	}
	if body["seq"].(float64) != 3 || body["success"].(bool) != true {
		t.Errorf("encoded request wrong: %v", body)
	}
	if !resp.Acknowledged || resp.NextSeq != 4 {
		t.Errorf("decoded response wrong: %+v", resp)
	}
}

func TestResetCursor_PropagatesError(t *testing.T) {
	f := &fakeDoCommander{err: errors.New("boom")}
	if err := ResetCursor(context.Background(), f); err == nil {
		t.Fatal("expected error to propagate")
	}
	if _, ok := f.gotCmd[contracts.VerbResetCursor]; !ok {
		t.Fatalf("request missing verb %q: %v", contracts.VerbResetCursor, f.gotCmd)
	}
}
