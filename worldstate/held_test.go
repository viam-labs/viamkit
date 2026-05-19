package worldstate

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
)

const eps = 1e-9

func TestGripperHeldBox_BasicShape(t *testing.T) {
	link, err := GripperHeldBox("gripper", "held-box", r3.Vector{X: 100, Y: 80, Z: 60})
	if err != nil {
		t.Fatal(err)
	}
	if link.Parent() != "gripper" {
		t.Errorf("Parent: got %q, want %q", link.Parent(), "gripper")
	}
	if link.Name() != "held-box" {
		t.Errorf("Name: got %q, want %q", link.Name(), "held-box")
	}
}

func TestGripperHeldBox_OffsetIsPositiveHalfHeight(t *testing.T) {
	// Box center hangs at gripper-local +H/2 (the dryrun-2 mis-diagnosis
	// repeatedly: -H/2 vs +H/2). 60 mm box → +30 mm offset.
	link, err := GripperHeldBox("gripper", "held-box", r3.Vector{X: 100, Y: 80, Z: 60})
	if err != nil {
		t.Fatal(err)
	}
	pose := link.Pose()
	if math.Abs(pose.Point().Z-30) > eps {
		t.Errorf("Z offset: got %v, want +30 (= H/2 for H=60)", pose.Point().Z)
	}
	if math.Abs(pose.Point().X) > eps || math.Abs(pose.Point().Y) > eps {
		t.Errorf("XY offset should be zero: got (%v, %v)", pose.Point().X, pose.Point().Y)
	}
}

func TestGripperHeldBox_GeometryShrinksByZPad(t *testing.T) {
	// Real box H=60; geometry Z dim should be 60 - 2*2.5 = 55.
	link, err := GripperHeldBox("gripper", "held-box", r3.Vector{X: 100, Y: 80, Z: 60})
	if err != nil {
		t.Fatal(err)
	}
	box := link.Geometry().ToProtobuf().GetBox()
	if box == nil {
		t.Fatal("expected Box geometry")
	}
	if math.Abs(box.DimsMm.Z-(60-2*DefaultHeldBoxZPadMM)) > eps {
		t.Errorf("Z dim: got %v, want %v (shrunk by 2*pad)",
			box.DimsMm.Z, 60-2*DefaultHeldBoxZPadMM)
	}
	if math.Abs(box.DimsMm.X-100) > eps || math.Abs(box.DimsMm.Y-80) > eps {
		t.Errorf("XY dims should be unchanged: got (%v, %v), want (100, 80)",
			box.DimsMm.X, box.DimsMm.Y)
	}
}

func TestGripperHeldBox_TinyBoxSkipsPad(t *testing.T) {
	// Box shorter than 2*pad — skip the pad rather than produce
	// negative dims.
	link, err := GripperHeldBox("gripper", "held-box", r3.Vector{X: 30, Y: 30, Z: 4})
	if err != nil {
		t.Fatal(err)
	}
	box := link.Geometry().ToProtobuf().GetBox()
	if box.DimsMm.Z != 4 {
		t.Errorf("tiny box: Z dim should be unshrunk (4); got %v", box.DimsMm.Z)
	}
}
