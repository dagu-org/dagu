//go:build windows

package docker

import (
	"fmt"
	"strings"
)

// parseVolumeSpec parses a volume specification string into a volumeSpec.
// Windows paths can contain colons (e.g., C:\path or //C:/path), so we need
// special handling to distinguish the drive letter colon from the separator.
//
// Supported formats:
//   - source:target[:mode]
//   - C:\path:target[:mode]
//   - C:/path:target[:mode]
//   - //C:/path:target[:mode] (Docker Toolbox style)
func parseVolumeSpec(vol string) (volumeSpec, error) {
	source, rest := splitSourcePath(vol)
	if rest == "" {
		return volumeSpec{}, fmt.Errorf("%w: %s", ErrInvalidVolumeFormat, vol)
	}

	// rest is now "target" or "target:mode"
	parts := strings.SplitN(rest, ":", 2)
	if parts[0] == "" {
		return volumeSpec{}, fmt.Errorf("%w: %s", ErrInvalidVolumeFormat, vol)
	}

	spec := volumeSpec{
		Source: source,
		Target: parts[0],
	}
	if len(parts) == 2 {
		spec.Mode = parts[1]
	}
	return spec, nil
}

// splitSourcePath extracts the source path from a volume specification,
// handling Windows drive letters and Docker Toolbox style paths.
// Returns the source path and the remaining string after the separator colon.
func splitSourcePath(vol string) (source, rest string) {
	// Handle Docker Toolbox style: //C:/path:target
	if strings.HasPrefix(vol, "//") && len(vol) > 4 && vol[3] == ':' {
		return splitAfterDriveLetter(vol, 2)
	}

	// Handle standard Windows path: C:\path:target or C:/path:target
	if len(vol) >= 2 && vol[1] == ':' {
		return splitAfterDriveLetter(vol, 0)
	}

	// Standard format without Windows drive letter
	idx := strings.Index(vol, ":")
	if idx == -1 {
		return vol, ""
	}
	return vol[:idx], vol[idx+1:]
}

// splitAfterDriveLetter splits the volume string after the Windows drive letter,
// starting the search from the given offset.
// For "C:\foo:target", offset=0, returns ("C:\foo", "target")
// For "//C:/foo:target", offset=2, returns ("//C:/foo", "target")
func splitAfterDriveLetter(vol string, offset int) (source, rest string) {
	// Find the colon after the drive letter (skip the one at offset+1)
	searchStart := offset + 2
	idx := strings.Index(vol[searchStart:], ":")
	if idx == -1 {
		return vol, ""
	}
	sepIdx := searchStart + idx
	return vol[:sepIdx], vol[sepIdx+1:]
}
