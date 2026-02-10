package core

import "fmt"

// OverlapPolicy controls how multiple catch-up runs are handled.
type OverlapPolicy string

const (
	// OverlapPolicySkip skips a new run if the previous is still running.
	// For catch-up: only the first missed interval runs; others are skipped
	// while it's in progress.
	OverlapPolicySkip OverlapPolicy = "skip"

	// OverlapPolicyAll executes all missed runs sequentially (queued, one
	// after another in chronological order). Ensures every missed interval
	// is processed.
	OverlapPolicyAll OverlapPolicy = "all"
)

// ParseOverlapPolicy parses a string into an OverlapPolicy.
// Empty string defaults to OverlapPolicySkip.
func ParseOverlapPolicy(s string) (OverlapPolicy, error) {
	switch s {
	case "", "skip":
		return OverlapPolicySkip, nil
	case "all":
		return OverlapPolicyAll, nil
	default:
		return "", fmt.Errorf("invalid overlapPolicy %q: must be \"skip\" or \"all\"", s)
	}
}
