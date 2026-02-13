package upgrade

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/Masterminds/semver/v3"
)

// ParseVersion parses a version string into a semver.Version.
func ParseVersion(s string) (*semver.Version, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty version string")
	}

	if s == "dev" || s == "0.0.0" {
		return nil, fmt.Errorf("cannot upgrade from development build (version: %s). Install a release version first: https://github.com/dagu-org/dagu/releases", s)
	}

	s = strings.TrimPrefix(s, "v")

	if idx := strings.Index(s, "-"); idx != -1 {
		suffix := s[idx+1:]
		if isNumeric(suffix) {
			s = s[:idx]
		}
	}

	v, err := semver.NewVersion(s)
	if err != nil {
		return nil, fmt.Errorf("invalid version format %q: %w", s, err)
	}

	return v, nil
}

// CompareVersions compares two versions.
// Returns: -1 if current < target, 0 if equal, 1 if current > target
func CompareVersions(current, target *semver.Version) int {
	return current.Compare(target)
}

// IsNewer returns true if target is newer than current.
func IsNewer(current, target *semver.Version) bool {
	return CompareVersions(current, target) < 0
}

// isNumeric checks if a string contains only digits.
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// NormalizeVersionTag ensures a version string has a 'v' prefix.
func NormalizeVersionTag(version string) string {
	version = strings.TrimSpace(version)
	if !strings.HasPrefix(version, "v") {
		return "v" + version
	}
	return version
}

// ExtractVersionFromTag extracts the version number from a tag like "v1.30.3".
func ExtractVersionFromTag(tag string) string {
	return strings.TrimPrefix(tag, "v")
}

// ValidateVersionTag rejects tags that contain path-traversal sequences,
// directory separators, or control characters.
func ValidateVersionTag(tag string) error {
	if strings.Contains(tag, "/") || strings.Contains(tag, "\\") {
		return fmt.Errorf("invalid version tag %q: contains path separator", tag)
	}
	if strings.Contains(tag, "..") {
		return fmt.Errorf("invalid version tag %q: contains path traversal", tag)
	}
	for _, r := range tag {
		if unicode.IsControl(r) {
			return fmt.Errorf("invalid version tag %q: contains control character", tag)
		}
	}
	return nil
}
