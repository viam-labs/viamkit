// Package kinematics holds pure helpers used by motion-heavy Viam
// modules: orientation math, trajectory shape handling, and a
// user-friendly mapping for the motion service's planner errors.
//
// Everything in this package is stateless and goroutine-safe. There
// are no constructors — call the functions directly.
package kinematics
