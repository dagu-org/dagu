package docker

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"
)

// volumeSpec represents a parsed volume specification
type volumeSpec struct {
	Source string
	Target string
	Mode   string // "ro", "rw", or empty (defaults to rw)
}

// parseVolumes parses volume specifications into bind mounts and volume mounts
func parseVolumes(workDir string, volumes []string) ([]string, []mount.Mount, error) {
	var binds []string
	var mounts []mount.Mount

	for _, vol := range volumes {
		spec, err := parseVolumeSpec(vol)
		if err != nil {
			return nil, nil, err
		}

		source := spec.Source
		target := spec.Target
		readOnly := false

		if spec.Mode == "ro" {
			readOnly = true
		} else if spec.Mode != "" && spec.Mode != "rw" {
			return nil, nil, fmt.Errorf("%w: invalid mode %s in %s", ErrInvalidVolumeFormat, spec.Mode, vol)
		}

		// Determine if it's a bind mount or volume
		if filepath.IsAbs(source) || strings.HasPrefix(source, ".") || strings.HasPrefix(source, "~") {
			if !filepath.IsAbs(source) {
				if workDir != "" && strings.HasPrefix(source, ".") {
					// Handle relative paths starting with "." or "./"
					if source == "." || source == "./" {
						source = workDir
					} else if strings.HasPrefix(source, "./") {
						source = filepath.Join(workDir, source[2:])
					} else {
						source = filepath.Join(workDir, source[1:])
					}
					source = filepath.Clean(source)
				} else {
					p, err := fileutil.ResolvePath(source)
					if err != nil {
						return nil, nil, fmt.Errorf("failed to resolve path %s: %w", source, err)
					}
					source = p
				}
			}

			// It's a bind mount
			bindStr := source + ":" + target
			if readOnly {
				bindStr += ":ro"
			} else {
				bindStr += ":rw"
			}
			binds = append(binds, bindStr)
		} else {
			// It's a named volume
			mnt := mount.Mount{
				Type:     mount.TypeVolume,
				Source:   source,
				Target:   target,
				ReadOnly: readOnly,
			}
			mounts = append(mounts, mnt)
		}
	}

	return binds, mounts, nil
}

// parsePorts parses port specifications into ExposedPorts and PortBindings
func parsePorts(ports []string) (nat.PortSet, nat.PortMap, error) {
	exposedPorts := make(nat.PortSet)
	portBindings := make(nat.PortMap)

	for _, portSpec := range ports {
		// Remove any whitespace
		portSpec = strings.TrimSpace(portSpec)

		// Split by colon to get components
		parts := strings.Split(portSpec, ":")

		var hostIP, hostPort, containerPort, proto string

		switch len(parts) {
		case 1:
			// Format: "80" or "80/tcp"
			containerPort = parts[0]
		case 2:
			// Format: "8080:80"
			hostPort = parts[0]
			containerPort = parts[1]
		case 3:
			// Format: "0.0.0.0:8080:80"
			hostIP = parts[0]
			hostPort = parts[1]
			containerPort = parts[2]
		default:
			return nil, nil, fmt.Errorf("%w: %s", ErrInvalidPortFormat, portSpec)
		}

		// Extract protocol if specified
		if strings.Contains(containerPort, "/") {
			protoParts := strings.Split(containerPort, "/")
			if len(protoParts) != 2 {
				return nil, nil, fmt.Errorf("%w: invalid protocol in %s", ErrInvalidPortFormat, portSpec)
			}
			containerPort = protoParts[0]
			proto = protoParts[1]
		} else {
			proto = "tcp" // Default to TCP
		}

		// Validate protocol
		if proto != "tcp" && proto != "udp" && proto != "sctp" {
			return nil, nil, fmt.Errorf("%w: invalid protocol %s in %s", ErrInvalidPortFormat, proto, portSpec)
		}

		// Create the nat.Port
		natPort := nat.Port(containerPort + "/" + proto)

		// Add to exposed ports
		exposedPorts[natPort] = struct{}{}

		// Add to port bindings if host port is specified
		if hostPort != "" {
			if hostIP == "" {
				hostIP = "0.0.0.0" // Default to all interfaces
			}

			portBindings[natPort] = []nat.PortBinding{
				{
					HostIP:   hostIP,
					HostPort: hostPort,
				},
			}
		}
	}

	return exposedPorts, portBindings, nil
}

// parseNetworkMode converts a network string to container.NetworkMode
func parseNetworkMode(network string) container.NetworkMode {
	// Standard network modes
	switch network {
	case "bridge", "host", "none":
		return container.NetworkMode(network)
	default:
		// Check if it's a container network reference
		if strings.HasPrefix(network, "container:") {
			return container.NetworkMode(network)
		}
		// Otherwise, it's a custom network name
		return container.NetworkMode(network)
	}
}

// isStandardNetworkMode checks if the network mode is a standard Docker network mode
func isStandardNetworkMode(network string) bool {
	return network == "bridge" || network == "host" || network == "none" ||
		strings.HasPrefix(network, "container:") || network == ""
}
