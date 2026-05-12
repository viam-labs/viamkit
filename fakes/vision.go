package fakes

import (
	"context"
	"image"
	"sync/atomic"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
	viz "go.viam.com/rdk/vision/viscapture"
	visionobj "go.viam.com/rdk/vision"
)

// Vision is an in-process fake of vision.Service for unit tests.
// Methods default to empty results (no detections, no
// classifications). Override the *Fn fields per test.
type Vision struct {
	name resource.Name

	DetectionsFromCameraFn func(ctx context.Context, cameraName string, extra map[string]interface{}) ([]objectdetection.Detection, error)
	DetectionsFn           func(ctx context.Context, img image.Image, extra map[string]interface{}) ([]objectdetection.Detection, error)

	ClassificationsFromCameraFn func(ctx context.Context, cameraName string, n int, extra map[string]interface{}) (classification.Classifications, error)
	ClassificationsFn           func(ctx context.Context, img image.Image, n int, extra map[string]interface{}) (classification.Classifications, error)

	GetObjectPointCloudsFn func(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*visionobj.Object, error)
	GetPropertiesFn        func(ctx context.Context, extra map[string]interface{}) (*vision.Properties, error)
	CaptureAllFromCameraFn func(ctx context.Context, cameraName string, opts viz.CaptureOptions, extra map[string]interface{}) (viz.VisCapture, error)

	classificationsFromCameraCalls atomic.Int32
	detectionsFromCameraCalls      atomic.Int32
}

// NewVision constructs a Vision fake with the given resource name.
func NewVision(name string) *Vision {
	return &Vision{name: vision.Named(name)}
}

// Helpful call counters for the two methods palletizer-style modules use most.
func (v *Vision) ClassificationsFromCameraCalls() int {
	return int(v.classificationsFromCameraCalls.Load())
}
func (v *Vision) DetectionsFromCameraCalls() int {
	return int(v.detectionsFromCameraCalls.Load())
}

// ---- resource.Resource ----

func (v *Vision) Name() resource.Name { return v.name }
func (v *Vision) Close(_ context.Context) error { return nil }
func (v *Vision) Reconfigure(_ context.Context, _ resource.Dependencies, _ resource.Config) error {
	return nil
}
func (v *Vision) DoCommand(_ context.Context, _ map[string]interface{}) (map[string]interface{}, error) {
	return nil, resource.ErrDoUnimplemented
}

// ---- vision.Service ----

func (v *Vision) DetectionsFromCamera(ctx context.Context, cameraName string, extra map[string]interface{}) ([]objectdetection.Detection, error) {
	v.detectionsFromCameraCalls.Add(1)
	if v.DetectionsFromCameraFn != nil {
		return v.DetectionsFromCameraFn(ctx, cameraName, extra)
	}
	return nil, nil
}

func (v *Vision) Detections(ctx context.Context, img image.Image, extra map[string]interface{}) ([]objectdetection.Detection, error) {
	if v.DetectionsFn != nil {
		return v.DetectionsFn(ctx, img, extra)
	}
	return nil, nil
}

func (v *Vision) ClassificationsFromCamera(ctx context.Context, cameraName string, n int, extra map[string]interface{}) (classification.Classifications, error) {
	v.classificationsFromCameraCalls.Add(1)
	if v.ClassificationsFromCameraFn != nil {
		return v.ClassificationsFromCameraFn(ctx, cameraName, n, extra)
	}
	return nil, nil
}

func (v *Vision) Classifications(ctx context.Context, img image.Image, n int, extra map[string]interface{}) (classification.Classifications, error) {
	if v.ClassificationsFn != nil {
		return v.ClassificationsFn(ctx, img, n, extra)
	}
	return nil, nil
}

func (v *Vision) GetObjectPointClouds(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*visionobj.Object, error) {
	if v.GetObjectPointCloudsFn != nil {
		return v.GetObjectPointCloudsFn(ctx, cameraName, extra)
	}
	return nil, nil
}

func (v *Vision) GetProperties(ctx context.Context, extra map[string]interface{}) (*vision.Properties, error) {
	if v.GetPropertiesFn != nil {
		return v.GetPropertiesFn(ctx, extra)
	}
	return &vision.Properties{}, nil
}

func (v *Vision) CaptureAllFromCamera(ctx context.Context, cameraName string, opts viz.CaptureOptions, extra map[string]interface{}) (viz.VisCapture, error) {
	if v.CaptureAllFromCameraFn != nil {
		return v.CaptureAllFromCameraFn(ctx, cameraName, opts, extra)
	}
	return viz.VisCapture{}, nil
}
