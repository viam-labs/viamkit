package worldstate

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

func TestNewBoxObstacle(t *testing.T) {
	pose := spatialmath.NewPoseFromPoint(r3.Vector{X: 100, Y: 200, Z: 50})
	geom, err := NewBoxObstacle(pose, r3.Vector{X: 50, Y: 60, Z: 40}, "box-1")
	if err != nil {
		t.Fatal(err)
	}
	if got := geom.Label(); got != "box-1" {
		t.Errorf("label: got %q, want %q", got, "box-1")
	}
}

func TestNewSphereObstacle(t *testing.T) {
	geom, err := NewSphereObstacle(spatialmath.NewZeroPose(), 25, "ball")
	if err != nil {
		t.Fatal(err)
	}
	if geom.Label() != "ball" {
		t.Errorf("label: got %q, want %q", geom.Label(), "ball")
	}
}

func TestHeldObject(t *testing.T) {
	heldGeom, err := NewBoxObstacle(spatialmath.NewZeroPose(),
		r3.Vector{X: 100, Y: 100, Z: 80}, "held-box")
	if err != nil {
		t.Fatal(err)
	}
	offset := spatialmath.NewPoseFromPoint(r3.Vector{Z: 40})
	link := HeldObject("gripper", offset, "held-box-frame", heldGeom)
	if link == nil {
		t.Fatal("HeldObject returned nil")
	}
	if link.Parent() != "gripper" {
		t.Errorf("parent: got %q, want %q", link.Parent(), "gripper")
	}
	if link.Name() != "held-box-frame" {
		t.Errorf("link name: got %q, want %q", link.Name(), "held-box-frame")
	}
}

func TestWorldObstacles(t *testing.T) {
	g1, _ := NewBoxObstacle(spatialmath.NewZeroPose(), r3.Vector{X: 10, Y: 10, Z: 10}, "a")
	g2, _ := NewSphereObstacle(spatialmath.NewZeroPose(), 5, "b")

	frames := WorldObstacles(g1, g2)
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(frames))
	}
	if frames[0].Parent() != "world" {
		t.Errorf("parent: got %q, want %q", frames[0].Parent(), "world")
	}
	if len(frames[0].Geometries()) != 2 {
		t.Errorf("geometries: got %d, want 2", len(frames[0].Geometries()))
	}
}

func TestWorldObstaclesEmptyReturnsNil(t *testing.T) {
	if frames := WorldObstacles(); frames != nil {
		t.Errorf("empty: got %v, want nil", frames)
	}
}

func TestCombinedObstaclesOnly(t *testing.T) {
	g, _ := NewBoxObstacle(spatialmath.NewZeroPose(), r3.Vector{X: 10, Y: 10, Z: 10}, "wall")
	ws, err := Combined(WorldObstacles(g), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(ws.Obstacles()); got != 1 {
		t.Errorf("obstacles: got %d, want 1", got)
	}
}

func TestCombinedHeldOnly(t *testing.T) {
	heldGeom, _ := NewBoxObstacle(spatialmath.NewZeroPose(),
		r3.Vector{X: 100, Y: 100, Z: 80}, "held")
	link := HeldObject("gripper", spatialmath.NewZeroPose(), "held-frame", heldGeom)
	ws, err := Combined(nil, []*referenceframe.LinkInFrame{link})
	if err != nil {
		t.Fatal(err)
	}
	_ = ws
}

func TestCombinedBoth(t *testing.T) {
	wallGeom, _ := NewBoxObstacle(spatialmath.NewZeroPose(),
		r3.Vector{X: 1000, Y: 1000, Z: 10}, "wall")
	heldGeom, _ := NewBoxObstacle(spatialmath.NewZeroPose(),
		r3.Vector{X: 100, Y: 100, Z: 80}, "held")
	link := HeldObject("gripper", spatialmath.NewZeroPose(), "held-frame", heldGeom)
	ws, err := Combined(WorldObstacles(wallGeom),
		[]*referenceframe.LinkInFrame{link})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(ws.Obstacles()); got != 1 {
		t.Errorf("obstacles: got %d, want 1", got)
	}
}
