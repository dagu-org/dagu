package container

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"

	"github.com/containerd/platforms"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/stringutil"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"github.com/go-viper/mapstructure/v2"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// Errors for container
var (
	ErrImageOrContainerShouldNotBeEmpty = errors.New("containerName or image must be specified")
	ErrImageRequired                    = errors.New("image is required")
	ErrInvalidVolumeFormat              = errors.New("invalid volume format")
	ErrInvalidPortFormat                = errors.New("invalid port format")
)

type Container struct {
	image         string
	platform      string
	containerName string
	pull          digraph.PullPolicy
	autoRemove    bool
	// containerConfig is the configuration for new container creation
	// See https://pkg.go.dev/github.com/docker/docker/api/types/container#Config
	containerConfig *container.Config
	// hostConfig is configuration for the container host
	// See https://pkg.go.dev/github.com/docker/docker/api/types/container#HostConfig
	hostConfig *container.HostConfig
	// networkConfig is configuration for the container network
	// See https://pkg.go.dev/github.com/docker/docker@v27.5.1+incompatible/api/types/network#NetworkingConfig
	networkConfig *network.NetworkingConfig
	// execOptions is configuration for exec in existing container
	// See https://pkg.go.dev/github.com/docker/docker/api/types/container#ExecOptions
	execOptions *container.ExecOptions
}

// ParseContainer parses digraph.Container into Container struct
func ParseContainer(ctx context.Context, ct digraph.Container) (*Container, error) {
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

	return &Container{
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

// ParseMapConfig parses executorConfig into Container struct
func ParseMapConfig(ctx context.Context, data map[string]any) (*Container, error) {
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

	return &Container{
		image:           ret.Image,
		platform:        ret.Platform,
		containerName:   ret.ContainerName,
		pull:            pull,
		containerConfig: &ret.Container,
		hostConfig:      &ret.Host,
		networkConfig:   &ret.Network,
		execOptions:     &ret.Exec,
		autoRemove:      autoRemove,
	}, nil
}

// Platform returns the platform of the container
func (c *Container) Platform(ctx context.Context, cli *client.Client) (specs.Platform, error) {
	// Extract platform from the current input and fallback to the current docker host platform.
	var platform specs.Platform
	if c.platform != "" {
		var err error
		platform, err = platforms.Parse(c.platform)
		if err != nil {
			return platform, fmt.Errorf("failed to parse platform %s: %w", c.platform, err)
		}
	} else {
		info, err := cli.Info(ctx)
		if err != nil {
			return platform, fmt.Errorf("failed to get current docker host info: %w", err)
		}
		platform.Architecture = info.Architecture
		platform.OS = info.OSType
		platform = platforms.Normalize(platform)
	}
	return platform, nil
}

func (c *Container) ShouldPullImage(ctx context.Context, cli *client.Client, platform *specs.Platform) (bool, error) {
	if c.pull == digraph.PullPolicyAlways {
		return true, nil
	}
	if c.pull == digraph.PullPolicyNever {
		return false, nil
	}

	// Loop through all locally available images that have the same reference with
	// the input image to check if we have the correct platform.
	filters := filters.NewArgs()
	filters.Add("reference", c.image)

	images, err := cli.ImageList(ctx, image.ListOptions{Filters: filters})
	if err != nil {
		return false, fmt.Errorf("failed to list local images %s: %w", c.image, err)
	}

	for _, summary := range images {
		inspect, err := cli.ImageInspect(ctx, summary.ID)
		if err != nil {
			return false, fmt.Errorf("failed to inspect image %s: %w", summary.ID, err)
		}
		if (platform.OS == inspect.Os) && (platform.Architecture == inspect.Architecture) && (platform.Variant == inspect.Variant) {
			// We have the correct image locally, no need to pull
			return false, nil
		}
	}

	// We don't have the correct image
	return true, nil
}

func (c *Container) Run(ctx context.Context, cmd []string, stdout, stderr io.Writer) error {
	cli, err := client.NewClientWithOpts(
		client.FromEnv, client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return err
	}

	defer func() {
		_ = cli.Close()
	}()

	if c.image == "" {
		return c.execInContainer(ctx, cli, cmd, stdout, stderr)
	}

	platform, err := c.Platform(ctx, cli)
	if err != nil {
		return err
	}

	pull, err := c.ShouldPullImage(ctx, cli, &platform)
	if err != nil {
		return err
	}
	if pull {
		reader, err := cli.ImagePull(ctx, c.image, image.PullOptions{Platform: platforms.Format(platform)})
		if err != nil {
			return err
		}
		// Output pull-image log to stderr instead of stdout
		_, err = io.Copy(stderr, reader)
		if err != nil {
			return err
		}
	}

	containerConfig := *c.containerConfig
	containerConfig.Cmd = cmd
	containerConfig.Image = c.image

	resp, err := cli.ContainerCreate(
		ctx, &containerConfig, c.hostConfig, c.networkConfig, &platform, c.containerName,
	)
	if err != nil {
		return err
	}

	var once sync.Once
	removeContainer := func() {
		if !c.autoRemove {
			return
		}
		once.Do(func() {
			if err := cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true}); err != nil {
				logger.Error(ctx, "docker executor: remove container", "err", err)
			}
		})
	}

	defer removeContainer()

	if err := cli.ContainerStart(
		ctx, resp.ID, container.StartOptions{},
	); err != nil {
		return err
	}

	return c.attachAndWait(ctx, cli, resp.ID, stdout, stderr)
}

func (c *Container) execInContainer(ctx context.Context, cli *client.Client, cmd []string, stdout, stderr io.Writer) error {
	// Check if containerInfo exists and is running
	containerInfo, err := cli.ContainerInspect(ctx, c.containerName)
	if err != nil {
		return fmt.Errorf("failed to inspect container %s: %w", c.containerName, err)
	}

	if !containerInfo.State.Running {
		return fmt.Errorf("container %s is not running", c.containerName)
	}

	// Create exec configuration
	execOpts := container.ExecOptions{
		User:         c.execOptions.User,
		Privileged:   c.execOptions.Privileged,
		Tty:          c.execOptions.Tty,
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
		Env:          c.execOptions.Env,
		WorkingDir:   c.execOptions.WorkingDir,
	}

	// Create exec instance
	containerID, err := cli.ContainerExecCreate(ctx, c.containerName, execOpts)
	if err != nil {
		return fmt.Errorf("failed to create exec: %w", err)
	}

	// Start exec instance
	resp, err := cli.ContainerExecAttach(ctx, containerID.ID, container.ExecAttachOptions{})
	if err != nil {
		return fmt.Errorf("failed to start exec: %w", err)
	}
	defer resp.Close()

	// Copy output
	go func() {
		if _, err := stdcopy.StdCopy(stdout, stderr, resp.Reader); err != nil {
			logger.Error(ctx, "docker executor: stdcopy", "err", err)
		}
	}()

	// Wait for exec to complete
	for {
		inspectResp, err := cli.ContainerExecInspect(ctx, containerID.ID)
		if err != nil {
			return fmt.Errorf("failed to inspect exec: %w", err)
		}

		if !inspectResp.Running {
			if inspectResp.ExitCode != 0 {
				return fmt.Errorf("exec failed with exit code: %d", inspectResp.ExitCode)
			}
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Continue waiting
		}
	}

	return nil
}

func (c *Container) attachAndWait(ctx context.Context, cli *client.Client, containerID string, stdout, stderr io.Writer) error {
	out, err := cli.ContainerLogs(
		ctx, containerID, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
		},
	)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := stdcopy.StdCopy(stdout, stderr, out); err != nil {
			logger.Error(ctx, "docker executor: stdcopy", "err", err)
		}
	}()

	statusCh, errCh := cli.ContainerWait(
		ctx, containerID, container.WaitConditionNotRunning,
	)
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return fmt.Errorf("exit status %v", status.StatusCode)
		}
	}

	// Wait for log copying to complete
	wg.Wait()

	return nil
}
