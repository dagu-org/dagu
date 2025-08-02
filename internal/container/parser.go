package container

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/stringutil"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/go-viper/mapstructure/v2"
)

// NewFromContainerConfig parses digraph.Container into Container struct
func NewFromContainerConfig(ct digraph.Container) (*Client, error) {
	// Validate required fields
	if ct.Image == "" {
		return nil, ErrImageRequired
	}

	// Initialize Docker configuration structs
	containerConfig := &container.Config{
		Image:      ct.Image,
		Env:        ct.Env,
		User:       ct.User,
		WorkingDir: ct.WorkDir,
	}

	hostConfig := &container.HostConfig{}
	networkConfig := &network.NetworkingConfig{}
	execOptions := &container.ExecOptions{}

	// Parse volumes
	if len(ct.Volumes) > 0 {
		binds, mounts, err := parseVolumes(ct.Volumes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse volumes: %w", err)
		}
		if len(binds) > 0 {
			hostConfig.Binds = binds
		}
		if len(mounts) > 0 {
			hostConfig.Mounts = mounts
		}
	}

	// Parse ports
	if len(ct.Ports) > 0 {
		exposedPorts, portBindings, err := parsePorts(ct.Ports)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ports: %w", err)
		}
		containerConfig.ExposedPorts = exposedPorts
		hostConfig.PortBindings = portBindings
	}

	// Parse network mode
	if ct.Network != "" {
		networkMode := parseNetworkMode(ct.Network)
		hostConfig.NetworkMode = networkMode

		// If it's a custom network, add it to the endpoints config
		if !isStandardNetworkMode(ct.Network) {
			networkConfig.EndpointsConfig = map[string]*network.EndpointSettings{
				ct.Network: {},
			}
		}
	}

	// autoRemove is the inverse of KeepContainer
	autoRemove := !ct.KeepContainer

	return &Client{
		image:           ct.Image,
		platform:        ct.Platform,
		pull:            ct.PullPolicy,
		autoRemove:      autoRemove,
		containerConfig: containerConfig,
		hostConfig:      hostConfig,
		networkConfig:   networkConfig,
		execOptions:     execOptions,
	}, nil
}

// NewFromMapConfig parses executorConfig into Container struct
func NewFromMapConfig(data map[string]any) (*Client, error) {
	ret := struct {
		Container     container.Config         `mapstructure:"container"`
		Host          container.HostConfig     `mapstructure:"host"`
		Network       network.NetworkingConfig `mapstructure:"network"`
		Exec          container.ExecOptions    `mapstructure:"exec"`
		AutoRemove    any                      `mapstructure:"autoRemove"`
		Pull          any                      `mapstructure:"pull"`
		Platform      string                   `mapstructure:"platform"`
		ContainerName string                   `mapstructure:"containerName"`
		Image         string                   `mapstructure:"image"`
	}{}

	md, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           &ret,
		WeaklyTypedInput: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %w", err)
	}
	if err := md.Decode(data); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	var autoRemove bool
	if ret.Host.AutoRemove {
		ret.Host.AutoRemove = false // Prevent removal by sdk
		autoRemove = true
	}

	pull := digraph.PullPolicyMissing
	if ret.Pull != nil {
		parsed, err := digraph.ParsePullPolicy(ret.Pull)
		if err != nil {
			return nil, err
		}
		pull = parsed
	}

	if ret.ContainerName == "" && ret.Image == "" {
		return nil, ErrImageOrContainerShouldNotBeEmpty
	}

	if ret.AutoRemove != nil {
		v, err := stringutil.ParseBool(ret.AutoRemove)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate autoRemove value: %w", err)
		}
		autoRemove = v
	}

	return &Client{
		image:           ret.Image,
		platform:        ret.Platform,
		nameOrID:        ret.ContainerName,
		pull:            pull,
		containerConfig: &ret.Container,
		hostConfig:      &ret.Host,
		networkConfig:   &ret.Network,
		execOptions:     &ret.Exec,
		autoRemove:      autoRemove,
	}, nil
}

// parseVolumes parses volume specifications into bind mounts and volume mounts
func parseVolumes(volumes []string) ([]string, []mount.Mount, error) {
	var binds []string
	var mounts []mount.Mount

	for _, vol := range volumes {
		parts := strings.Split(vol, ":")
		if len(parts) < 2 || len(parts) > 3 {
			return nil, nil, fmt.Errorf("%w: %s", ErrInvalidVolumeFormat, vol)
		}

		source := parts[0]
		target := parts[1]
		readOnly := false

		// Check for read-only flag
		if len(parts) == 3 {
			if parts[2] == "ro" {
				readOnly = true
			} else if parts[2] != "rw" {
				return nil, nil, fmt.Errorf("%w: invalid mode %s in %s", ErrInvalidVolumeFormat, parts[2], vol)
			}
		}

		// Determine if it's a bind mount or volume
		if filepath.IsAbs(source) || strings.HasPrefix(source, ".") || strings.HasPrefix(source, "~") {
			// It's a bind mount
			bindStr := vol
			if len(parts) == 2 {
				// Add default rw mode if not specified
				bindStr = source + ":" + target + ":rw"
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
