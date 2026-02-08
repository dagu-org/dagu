package duration

import (
	"fmt"
	"regexp"
	"time"
)

var dayPattern = regexp.MustCompile(`(\d+)d`)

// Parse parses a duration string that supports the standard Go duration format
// plus a 'd' suffix for days (e.g., "2d12h", "1d30m", "90m").
// It converts day units to 24h equivalents before delegating to time.ParseDuration.
func Parse(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}

	// Expand 'd' units to hours: e.g. "2d" -> "48h"
	expanded := dayPattern.ReplaceAllStringFunc(s, func(match string) string {
		var days int
		_, _ = fmt.Sscanf(match, "%dd", &days)
		return fmt.Sprintf("%dh", days*24)
	})

	d, err := time.ParseDuration(expanded)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}

	if d < 0 {
		return 0, fmt.Errorf("negative duration not allowed: %q", s)
	}

	return d, nil
}
