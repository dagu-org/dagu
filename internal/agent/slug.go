package agent

import (
	"fmt"
	"regexp"
	"strings"
)

// validSlugRegexp matches a valid slug: lowercase alphanumeric segments separated by hyphens.
var validSlugRegexp = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// maxSlugLength is the maximum allowed length for a slug ID.
const maxSlugLength = 128

// validateSlugID validates that id is a safe, well-formed slug identifier.
// It must be a non-empty slug (lowercase alphanumeric segments separated by hyphens)
// and at most 128 characters. This prevents path traversal and other injection attacks.
func validateSlugID(id string, errSentinel error) error {
	if id == "" {
		return errSentinel
	}
	if len(id) > maxSlugLength {
		return fmt.Errorf("%w: exceeds maximum length of %d", errSentinel, maxSlugLength)
	}
	if !validSlugRegexp.MatchString(id) {
		return fmt.Errorf("%w: must match pattern [a-z0-9]+(-[a-z0-9]+)*", errSentinel)
	}
	return nil
}

var slugRegexp = regexp.MustCompile(`[^a-z0-9]+`)

// GenerateSlugID creates a URL-friendly slug from a name.
// E.g., "Claude Opus 4.6" -> "claude-opus-4-6"
func GenerateSlugID(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugRegexp.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// maxSuffixLen reserves room for collision suffixes like "-999999999".
const maxSuffixLen = 10

// UniqueID generates a unique slug ID, appending "-2", "-3" etc. on collision.
// The fallback is used when the name produces an empty slug (e.g. only special characters).
// The result is guaranteed to not exceed maxSlugLength.
func UniqueID(name string, existingIDs map[string]struct{}, fallback string) string {
	base := GenerateSlugID(name)
	if base == "" {
		base = fallback
	}
	if len(base) > maxSlugLength-maxSuffixLen {
		base = base[:maxSlugLength-maxSuffixLen]
	}
	id := base
	if _, exists := existingIDs[id]; !exists {
		return id
	}
	for i := 2; ; i++ {
		id = fmt.Sprintf("%s-%d", base, i)
		if _, exists := existingIDs[id]; !exists {
			return id
		}
	}
}
