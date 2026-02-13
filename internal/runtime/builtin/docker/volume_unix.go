//go:build !windows

package docker

import (
	"fmt"
	"strings"
)

// parseVolumeSpec parses a volume specification string into a volumeSpec.
// Unix format: source:target[:mode]
func parseVolumeSpec(vol string) (volumeSpec, error) {
	parts := strings.Split(vol, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return volumeSpec{}, fmt.Errorf("%w: %s", ErrInvalidVolumeFormat, vol)
	}

	spec := volumeSpec{
		Source: parts[0],
		Target: parts[1],
	}
	if len(parts) == 3 {
		spec.Mode = parts[2]
	}
	return spec, nil
}
