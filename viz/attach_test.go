package viz

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
)

func TestAttachToGripper_OffsetIsPositiveHalfHeight(t *testing.T) {
	tr := AttachToGripper("held", "gripper", r3.Vector{X: 100, Y: 80, Z: 60}, Color{})
	pose := tr.PoseInObserverFrame.Pose
	if math.Abs(pose.Z-30) > eps {
		t.Errorf("Z offset: got %v, want +30 (H/2 for H=60)", pose.Z)
	}
	if pose.X != 0 || pose.Y != 0 {
		t.Errorf("XY should be zero: got (%v, %v)", pose.X, pose.Y)
	}
}

func TestAttachToGripper_ParentIsGripperFrame(t *testing.T) {
	tr := AttachToGripper("held", "my-gripper", r3.Vector{X: 100, Y: 80, Z: 60}, Color{})
	if tr.PoseInObserverFrame.ReferenceFrame != "my-gripper" {
		t.Errorf("ObserverFrame: got %q, want %q",
			tr.PoseInObserverFrame.ReferenceFrame, "my-gripper")
	}
}

func TestAttachToGripper_DimsPropagate(t *testing.T) {
	tr := AttachToGripper("held", "gripper", r3.Vector{X: 100, Y: 80, Z: 60}, Color{})
	box := tr.PhysicalObject.GetBox()
	if box == nil {
		t.Fatal("expected Box geometry")
	}
	if box.DimsMm.X != 100 || box.DimsMm.Y != 80 || box.DimsMm.Z != 60 {
		t.Errorf("dims: got (%v, %v, %v), want (100, 80, 60)",
			box.DimsMm.X, box.DimsMm.Y, box.DimsMm.Z)
	}
}

func TestAttachToGripper_ColorMetadata(t *testing.T) {
	tr := AttachToGripper("held", "gripper", r3.Vector{X: 100, Y: 80, Z: 60},
		Color{R: 176, G: 136, B: 80, Opacity: 0.9})
	if tr.Metadata == nil {
		t.Fatal("expected metadata when Color set")
	}
}
