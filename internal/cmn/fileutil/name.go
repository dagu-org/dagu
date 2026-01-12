package fileutil

import (
	"regexp"
)

const (
	// MaxSafeNameLength is the maximum length of a safe filename
	MaxSafeNameLength = 100
)

var (
	// Only allow alphanumeric characters, underscores, and hyphens
	allowedCharRegex = regexp.MustCompile(`[^a-zA-Z0-9_\-]`)
)

// SafeName converts a string to a safe filename containing only alphanumeric characters,
// underscores, and hyphens
func SafeName(str string) string {
	// Replace any character not in [a-zA-Z0-9_\-] with underscore
	safe := allowedCharRegex.ReplaceAllString(str, "_")

	// Truncate to a safe length
	runes := []rune(safe)
	if len(runes) > MaxSafeNameLength {
		safe = string(runes[:MaxSafeNameLength])
	}

	return safe
}
