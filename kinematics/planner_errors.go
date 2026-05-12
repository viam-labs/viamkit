package kinematics

import "strings"

// FriendlyPlannerError translates a motion-service planner error
// string into a human-readable explanation, with the supplied label
// substituted into the message where appropriate (e.g. "place_start",
// "pickup_home", or whatever the target is named).
//
// Returns "unknown failure" if raw is empty; falls through to a
// best-effort trimmed version of raw if no known pattern matches.
//
// The mapped messages are written for motion-planning workflows that
// hold a wrist-orientation constraint while planning (the common
// pick-and-place pattern). If your module plans without an
// orientation lock, the "wrist held steady" framing in some messages
// may read oddly — consumers can wrap and substitute their own text
// where it matters, or call this only for orientation-locked plans.
//
// Patterns recognized:
//   - "cbirrt timeout" / "deadline exceeded" → planner timed out
//   - "no paths found" / "no valid path" → no collision-free path
//   - "unreachable" / "ik solve failed" / "no ik" → out of joint limits
//   - "collision" → collision with arm or obstacle
//   - "partial plan" → planner found a partial path
//   - "skipped" → upstream stage failed
//   - "could not extract" → internal trajectory-decode error
//
// All matching is case-insensitive.
func FriendlyPlannerError(raw, label string) string {
	if raw == "" {
		return "unknown failure"
	}
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "cbirrt timeout") || strings.Contains(lower, "deadline exceeded"):
		return "Planner timed out searching for a collision-free path with the wrist orientation held steady — this slot is likely reachable only with a wrist flip."
	case strings.Contains(lower, "no paths found") || strings.Contains(lower, "no valid path"):
		return "No collision-free path exists to the " + label + " under the wrist-orientation lock."
	case strings.Contains(lower, "unreachable") || strings.Contains(lower, "ik solve failed") || strings.Contains(lower, "no ik"):
		return "Arm can't physically reach the " + label + " — outside joint limits or workspace."
	case strings.Contains(lower, "collision"):
		return "All planned paths collide with something (arm self-collision or obstacle)."
	case strings.Contains(lower, "partial plan"):
		return "Planner found a partial path but couldn't reach the " + label + " — the remainder of the trajectory is infeasible."
	case strings.Contains(lower, "skipped"):
		return "Skipped: the approach pose already failed, so the final pose couldn't be tested."
	case strings.Contains(lower, "could not extract"):
		return "Internal error: couldn't read the planner's trajectory output."
	}
	// No known pattern — trim the gRPC noise ("rpc error: code = X desc = ...")
	// so the underlying message at least reads cleanly.
	trimmed := strings.TrimPrefix(raw, "rpc error: ")
	if idx := strings.Index(trimmed, " desc = "); idx >= 0 {
		trimmed = trimmed[idx+len(" desc = "):]
	}
	return trimmed
}
