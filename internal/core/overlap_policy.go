package core

import (
	"fmt"
	"strings"
)

// OverlapPolicy controls behavior when a new run is triggered while a previous run is still active.
type OverlapPolicy string

const (
	// OverlapPolicySkip skips a new run if the previous is still running.
	OverlapPolicySkip OverlapPolicy = "skip"

	// OverlapPolicyAll queues all runs and executes them sequentially in chronological order.
	OverlapPolicyAll OverlapPolicy = "all"
)

// ParseOverlapPolicy parses a string into an OverlapPolicy.
// Empty string defaults to OverlapPolicySkip.
func ParseOverlapPolicy(s string) (OverlapPolicy, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "skip":
		return OverlapPolicySkip, nil
	case "all":
		return OverlapPolicyAll, nil
	default:
		return "", fmt.Errorf("invalid overlapPolicy %q: must be \"skip\" or \"all\"", s)
	}
}
