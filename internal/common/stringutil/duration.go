package stringutil

import (
	"fmt"
	"time"
)

// FormatDuration formats a duration into a human-readable string.
// It handles negative durations by prepending a "-" sign.
// Examples:
//   - 500ms -> "500ms"
//   - 1.5s -> "1.5s"
//   - 2m30s -> "2m 30s"
//   - 1h30m -> "1h 30m"
//   - -500ms -> "-500ms"
//   - -2m30s -> "-2m 30s"
func FormatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}

	// Handle negative durations
	if d < 0 {
		return "-" + FormatDuration(-d)
	}

	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	} else if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", mins, secs)
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, mins)
}
