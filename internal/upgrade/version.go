package upgrade

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/dagu-org/dagu/internal/cmn/config"
)

// ParseVersion parses a version string into a semver.Version.
// Handles formats: "v1.2.3", "1.2.3", "v1.2.3-rc.1", "dev"
func ParseVersion(s string) (*semver.Version, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty version string")
	}

	// Handle "dev" or other development versions
	if s == "dev" || s == "0.0.0" {
		return nil, fmt.Errorf("development version")
	}

	// Remove 'v' prefix if present
	s = strings.TrimPrefix(s, "v")

	// Handle build-time format like "1.30.3-260204123456"
	// Extract just the semver portion (before any build metadata that's not semver-compliant)
	if idx := strings.Index(s, "-"); idx != -1 {
		suffix := s[idx+1:]
		// Check if suffix looks like a build timestamp (all digits)
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

// CurrentVersion returns the current version from config.Version.
func CurrentVersion() (*semver.Version, error) {
	return ParseVersion(config.Version)
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

// versionPattern matches semver-like version strings.
var versionPattern = regexp.MustCompile(`^v?\d+\.\d+\.\d+`)

// LooksLikeVersion checks if a string looks like a version number.
func LooksLikeVersion(s string) bool {
	return versionPattern.MatchString(s)
}
