package viz

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	wsspb "go.viam.com/api/service/worldstatestore/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"
	"go.viam.com/rdk/spatialmath"
)

func rn(name string) resource.Name {
	return resource.NewName(worldstatestore.API, name)
}

func drainOne(t *testing.T, s *Store) worldstatestore.TransformChange {
	t.Helper()
	select {
	case ev := <-s.changeChan:
		return ev
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for change event")
		return worldstatestore.TransformChange{}
	}
}

func tx(uuid string) *commonpb.Transform {
	return Box{
		UUID:   uuid,
		Pose:   spatialmath.NewZeroPose(),
		DimsMM: r3.Vector{X: 10, Y: 10, Z: 10},
	}.ToTransform()
}

func TestStoreSetAdd(t *testing.T) {
	s := NewStore()
	s.Set(tx("a"))
	if s.Len() != 1 {
		t.Errorf("Len after Set: got %d, want 1", s.Len())
	}
	tr, err := s.GetTransform(context.Background(), []byte("a"), nil)
	if err != nil {
		t.Fatalf("GetTransform: %v", err)
	}
	if string(tr.Uuid) != "a" {
		t.Errorf("UUID: got %q", tr.Uuid)
	}
}

func TestStoreSetReplaceEmitsUpdated(t *testing.T) {
	s := NewStore()
	s.Set(tx("a"))
	drainOne(t, s) // consume the ADDED

	s.Set(tx("a")) // same UUID -> UPDATED
	change := drainOne(t, s)
	if change.ChangeType != wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_UPDATED {
		t.Errorf("expected UPDATED on replace, got %v", change.ChangeType)
	}
}

func TestStoreRemoveUnknownIsNoop(t *testing.T) {
	s := NewStore()
	s.Remove("nope")
	if s.Len() != 0 {
		t.Errorf("Len: got %d, want 0", s.Len())
	}
	select {
	case c := <-s.changeChan:
		t.Errorf("expected no event for unknown remove, got %v", c)
	case <-time.After(10 * time.Millisecond):
		// ok
	}
}

func TestStoreRemoveExistingEmits(t *testing.T) {
	s := NewStore()
	s.Set(tx("a"))
	drainOne(t, s)
	s.Remove("a")
	change := drainOne(t, s)
	if change.ChangeType != wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED {
		t.Errorf("expected REMOVED, got %v", change.ChangeType)
	}
	if s.Len() != 0 {
		t.Errorf("Len after Remove: got %d, want 0", s.Len())
	}
}

func TestStoreClearEmitsAll(t *testing.T) {
	s := NewStore()
	for _, u := range []string{"a", "b", "c"} {
		s.Set(tx(u))
		drainOne(t, s) // consume ADDED
	}
	s.Clear()
	seen := map[string]bool{}
	for i := 0; i < 3; i++ {
		c := drainOne(t, s)
		if c.ChangeType != wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED {
			t.Errorf("Clear[%d] type: got %v, want REMOVED", i, c.ChangeType)
		}
		seen[string(c.Transform.Uuid)] = true
	}
	if len(seen) != 3 {
		t.Errorf("Clear emitted %d distinct UUIDs, want 3 (got %v)", len(seen), seen)
	}
	if s.Len() != 0 {
		t.Errorf("Len after Clear: got %d, want 0", s.Len())
	}
}

func TestStoreListUUIDs(t *testing.T) {
	s := NewStore()
	for _, u := range []string{"a", "b"} {
		s.Set(tx(u))
	}
	uuids, err := s.ListUUIDs(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(uuids) != 2 {
		t.Errorf("ListUUIDs: got %d, want 2", len(uuids))
	}
}

func TestStoreGetUnknownErrors(t *testing.T) {
	s := NewStore()
	if _, err := s.GetTransform(context.Background(), []byte("nope"), nil); err == nil {
		t.Error("expected error for unknown uuid")
	}
}

func TestStoreSetEmptyUUIDIsNoop(t *testing.T) {
	s := NewStore()
	s.Set(&commonpb.Transform{Uuid: nil})
	if s.Len() != 0 {
		t.Errorf("Len after empty-UUID Set: got %d, want 0", s.Len())
	}
}

func TestStoreOnDroppedFires(t *testing.T) {
	var dropped atomic.Int32
	// Buffer 0 so any unconsumed emit drops immediately.
	s := NewStore(WithChangeBufferSize(0), OnDropped(func(_ string) {
		dropped.Add(1)
	}))
	for i := 0; i < 5; i++ {
		s.Set(tx("x"))
	}
	// 4 of the 5 should have dropped (no consumer).
	if d := dropped.Load(); d < 1 {
		t.Errorf("OnDropped fired %d times, want >= 1 (buffer was 0, no consumer)", d)
	}
}

func TestStoreServiceDoCommandRemove(t *testing.T) {
	s := NewStoreService(rn("x"))
	s.Set(tx("a"))
	s.Set(tx("b"))
	resp, err := s.DoCommand(context.Background(), map[string]interface{}{"remove": "a"})
	if err != nil {
		t.Fatalf("DoCommand remove: %v", err)
	}
	if resp["removed"].(int) != 1 {
		t.Errorf("remove count: got %v, want 1", resp["removed"])
	}
	if s.Len() != 1 {
		t.Errorf("Len: got %d, want 1", s.Len())
	}
}

func TestStoreServiceDoCommandRemoveArray(t *testing.T) {
	s := NewStoreService(rn("x"))
	s.Set(tx("a"))
	s.Set(tx("b"))
	s.Set(tx("c"))
	resp, err := s.DoCommand(context.Background(), map[string]interface{}{
		"remove": []interface{}{"a", "b"},
	})
	if err != nil {
		t.Fatalf("DoCommand: %v", err)
	}
	if resp["removed"].(int) != 2 {
		t.Errorf("remove count: got %v, want 2", resp["removed"])
	}
	if s.Len() != 1 {
		t.Errorf("Len: got %d, want 1", s.Len())
	}
}

func TestStoreServiceDoCommandClear(t *testing.T) {
	s := NewStoreService(rn("x"))
	s.Set(tx("a"))
	resp, err := s.DoCommand(context.Background(), map[string]interface{}{"clear": true})
	if err != nil {
		t.Fatalf("DoCommand: %v", err)
	}
	if !resp["cleared"].(bool) {
		t.Errorf("cleared: got %v, want true", resp["cleared"])
	}
	if s.Len() != 0 {
		t.Errorf("Len after clear: got %d, want 0", s.Len())
	}
}

func TestStoreServiceDoCommandLen(t *testing.T) {
	s := NewStoreService(rn("x"))
	s.Set(tx("a"))
	s.Set(tx("b"))
	resp, err := s.DoCommand(context.Background(), map[string]interface{}{"len": true})
	if err != nil {
		t.Fatalf("DoCommand: %v", err)
	}
	if resp["count"].(int) != 2 {
		t.Errorf("count: got %v, want 2", resp["count"])
	}
}

func TestStoreServiceDoCommandUnknownErrors(t *testing.T) {
	s := NewStoreService(rn("x"))
	if _, err := s.DoCommand(context.Background(), map[string]interface{}{"frob": true}); err == nil {
		t.Error("expected error for unknown verb")
	}
}

