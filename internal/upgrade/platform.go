package upgrade

import (
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"
)

// Platform represents the current operating system and architecture.
type Platform struct {
	OS   string // runtime.GOOS: linux, darwin, freebsd, openbsd, windows
	Arch string // Mapped from runtime.GOARCH to release asset naming
}

// Detect returns the current platform.
func Detect() Platform {
	p := Platform{OS: runtime.GOOS}

	// Map GOARCH to release asset naming
	switch runtime.GOARCH {
	case "amd64":
		p.Arch = "amd64"
	case "arm64":
		p.Arch = "arm64"
	case "386":
		p.Arch = "386"
	case "arm":
		// ARM versions are differentiated by GOARM env var
		goarm := os.Getenv("GOARM")
		switch goarm {
		case "5", "6":
			p.Arch = "armv6"
		default:
			// Default to armv7 as it's more common
			p.Arch = "armv7"
		}
	case "ppc64le":
		p.Arch = "ppc64le"
	case "s390x":
		p.Arch = "s390x"
	default:
		p.Arch = runtime.GOARCH
	}

	return p
}

// AssetName returns the expected filename for this platform and version.
// Format: boltbase_<version_without_v>_<os>_<arch>.tar.gz
func (p Platform) AssetName(version string) string {
	v := strings.TrimPrefix(version, "v")
	return fmt.Sprintf("boltbase_%s_%s_%s.tar.gz", v, p.OS, p.Arch)
}

// String returns a human-readable representation of the platform.
func (p Platform) String() string {
	return fmt.Sprintf("%s/%s", p.OS, p.Arch)
}

// IsSupported checks if the current platform is supported for self-upgrade.
func (p Platform) IsSupported() bool {
	supportedPlatforms := map[string][]string{
		"darwin":  {"amd64", "arm64"},
		"linux":   {"386", "amd64", "arm64", "armv6", "armv7", "ppc64le", "s390x"},
		"freebsd": {"386", "amd64", "arm64", "armv6", "armv7"},
		"openbsd": {"386", "amd64", "arm64", "armv6", "armv7"},
		"windows": {"386", "amd64", "arm64", "armv6", "armv7"},
	}

	arches, ok := supportedPlatforms[p.OS]
	if !ok {
		return false
	}

	return slices.Contains(arches, p.Arch)
}

// SupportedPlatformsMessage returns a message listing all supported platforms.
func SupportedPlatformsMessage() string {
	return `Supported platforms:
  darwin:  amd64, arm64
  linux:   386, amd64, arm64, armv6, armv7, ppc64le, s390x
  freebsd: 386, amd64, arm64, armv6, armv7
  openbsd: 386, amd64, arm64, armv6, armv7
  windows: 386, amd64, arm64, armv6, armv7`
}
