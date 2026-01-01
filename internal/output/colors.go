// Package output provides tree-structured rendering for DAG execution status.
package output

import (
	"strings"
	"unicode"

	"github.com/dagu-org/dagu/internal/core"
)

// StatusText returns human-readable status text for a DAG status.
func StatusText(status core.Status) string {
	// Convert snake_case to Title Case (e.g., "partially_succeeded" -> "Partially Succeeded")
	s := status.String()
	s = strings.ReplaceAll(s, "_", " ")
	return toTitleCase(s)
}

// toTitleCase capitalizes the first letter of each word.
func toTitleCase(s string) string {
	var result strings.Builder
	capitalizeNext := true
	for _, r := range s {
		if r == ' ' {
			capitalizeNext = true
			result.WriteRune(r)
		} else if capitalizeNext {
			result.WriteRune(unicode.ToUpper(r))
			capitalizeNext = false
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}
