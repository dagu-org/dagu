package fileutil

import (
	"regexp"
	"strings"
	"unicode"
)

// https://github.com/sindresorhus/filename-reserved-regex/blob/master/index.js

var (
	reservedCharRegex = regexp.MustCompile(
		`[<>:"/\\|!?*.\x00-\x1F]`,
	)
	reservedNamesRegex = regexp.MustCompile(
		`(?i)^(con|prn|aux|nul|com[0-9]|lpt[0-9])$`,
	)
)

const (
	// MaxSafeNameLength is the maximum length of a safe filename
	MaxSafeNameLength = 100
)

// SafeName converts a string to a safe filename
func SafeName(str string) string {
	// Convert to lowercase and remove non-allowed characters
	safe := strings.ToLower(str)

	// Replace reserved names
	safe = reservedCharRegex.ReplaceAllString(safe, "_")

	// Replace reserved Windows names
	safe = reservedNamesRegex.ReplaceAllStringFunc(safe, func(s string) string {
		return "_" + s + "_"
	})

	// Replace spaces and non-printable characters with underscores
	safe = strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) && r != ' ' {
			return r
		}
		return '_'
	}, safe)

	// Truncate to a safe length (100 is generally safe)
	// Use runes to safely truncate multi-byte characters
	runes := []rune(safe)
	if len(runes) >= MaxSafeNameLength {
		safe = string(runes[:MaxSafeNameLength])
	}

	// Ensure the last character is not a partial Unicode character
	return strings.TrimRight(safe, "\uFFFD")
}
