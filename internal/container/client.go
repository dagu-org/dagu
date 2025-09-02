package container

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/containerd/platforms"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/stringutil"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/go-viper/mapstructure/v2"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// Errors for container
var (
	ErrImageOrContainerShouldNotBeEmpty = errors.New("containerName or image must be specified")
	ErrImageRequired                    = errors.New("image is required")
	ErrInvalidVolumeFormat              = errors.New("invalid volume format")
	ErrInvalidPortFormat                = errors.New("invalid port format")
	ErrContainerIsNotRunning            = errors.New("container is not running")
)

type Client struct {
	image      string
	platform   string
	platformO  specs.Platform
	id         string
	pull       digraph.PullPolicy
	autoRemove bool
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
	execOptions  *container.ExecOptions
	mu           sync.Mutex
	cli          *client.Client
	keepAliveTmp string
	// authManager handles registry authentication
	authManager *RegistryAuthManager

	cancelMu sync.Mutex
	cancel   func()
}

// ExecOptions specifies options to execute commands in the container.
type ExecOptions struct {
	WorkingDir string // Working directory
}

// Init initialize the client
func (c *Client) Init(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	c.cli = cli

	platform, err := c.getPlatform(ctx, cli)
	if err != nil {
		return err
	}
	c.platformO = platform

	return nil
}

// Destroy destroys the client
func (c *Client) Close(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.id != "" && c.autoRemove {
		if err := c.cli.ContainerRemove(ctx, c.id, container.RemoveOptions{Force: true}); err != nil {
			logger.Error(ctx, "docker executor: remove container", "err", err)
		}
		c.id = ""
	}

	_ = c.cli.Close()
	c.cli = nil
}

// Exec executes the command in the running container
func (c *Client) Exec(ctx context.Context, cmd []string, stdout, stderr io.Writer, opts ExecOptions) (int, error) {
	c.mu.Lock()
	if c.id == "" {
		c.mu.Unlock()
		return 1, ErrContainerIsNotRunning
	}
	cli := c.cli
	c.mu.Unlock()

	return c.execInContainer(ctx, cli, cmd, stdout, stderr, opts)
}

// CreateContainerKeepAlive creates the container that lives while the DAG running
func (c *Client) CreateContainerKeepAlive(ctx context.Context) error {
	if c.id != "" {
		return fmt.Errorf("container already exists. id=%s", c.id)
	}

	// Use the command to keep alive the container
	var cmd []string
	if len(c.containerConfig.Cmd) == 0 {
		// Detect if we're running in docker-in-docker environment
		isDockerInDocker := c.isDockerInDocker()

		if isDockerInDocker {
			// We're in a container, use a simple sleep command as fallback
			logger.Info(ctx, "Detected docker-in-docker environment, using sleep for keepalive")
			// Use a shell command that sleeps indefinitely
			// Most images have sh, and this works across platforms
			cmd = []string{"sh", "-c", "while true; do sleep 86400; done"}
		} else {
			// Standard environment, use the keepalive binary
			hostPath, err := GetKeepaliveFile(c.platformO)
			if err != nil {
				// Fallback to sleep if keepalive binary fails
				logger.Warn(ctx, "Failed to get keepalive binary, using sleep fallback", "err", err)
				cmd = []string{"sh", "-c", "while true; do sleep 86400; done"}
			} else {
				c.keepAliveTmp = hostPath

				// Setup the volume bind for the keepalive binary
				targetPath := "/__dagu_runner/keepalive"
				bind := hostPath + ":" + targetPath + ":ro"
				c.hostConfig.Binds = append(c.hostConfig.Binds, bind)
				cmd = []string{targetPath}
			}
		}
	}

	// Set init true to prevent zombie subprocess issues
	init := true
	c.hostConfig.Init = &init

	ctx, cancel := context.WithCancel(ctx)
	c.cancelMu.Lock()
	c.cancel = cancel
	c.cancelMu.Unlock()

	id, err := c.startNewContainer(ctx, c.cli, cmd)
	if err != nil {
		return fmt.Errorf("failed to start a new container: %w", err)
	}

	c.id = id

	return nil
}

// StopContainerKeepAlive stops the container running keep alive command
func (c *Client) StopContainerKeepAlive(ctx context.Context) {
	c.cancelMu.Lock()
	defer c.cancelMu.Unlock()

	if c.cancel == nil {
		return
	}
	c.cancel()
	c.cancel = nil

	if c.id == "" {
		return
	}

	if err := c.cli.ContainerStop(ctx, c.id, container.StopOptions{}); err != nil {
		logger.Error(ctx, "docker executor: stop container", "err", err)
	}
	// Remove the temporary keep alive file if it exists
	if err := os.Remove(c.keepAliveTmp); err != nil && !os.IsNotExist(err) {
		logger.Error(ctx, "docker executor: remove keep alive file", "err", err)
	}
	c.keepAliveTmp = ""
}

// Run executes the command in the container and returns exit code
func (c *Client) Run(ctx context.Context, cmd []string, stdout, stderr io.Writer) (int, error) {
	c.mu.Lock()
	if c.id != "" {
		c.mu.Unlock()
		return c.execInContainer(ctx, c.cli, cmd, stdout, stderr, ExecOptions{})
	}

	id, err := c.startNewContainer(ctx, c.cli, cmd)
	if err != nil {
		c.mu.Unlock()
		return 1, fmt.Errorf("failed to start a new container: %w", err)
	}

	c.id = id
	c.mu.Unlock()

	var once sync.Once
	defer func() {
		if !c.autoRemove {
			return
		}

		once.Do(func() {
			c.mu.Lock()
			id := c.id
			c.mu.Unlock()

			if err := c.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}); err != nil {
				logger.Error(ctx, "docker executor: remove container", "err", err)
			}
		})
	}()

	return c.attachAndWait(ctx, c.cli, id, stdout, stderr)
}

func (c *Client) startNewContainer(ctx context.Context, cli *client.Client, cmd []string) (string, error) {
	pull, err := c.shouldPullImage(ctx, cli, &c.platformO)
	if err != nil {
		return "", err
	}

	logger.Info(ctx, "Creating a new container", "platform", c.platform, "image", c.image)

	if pull {
		logger.Infof(ctx, "Pulling the image %q", c.image)

		// Get pull options with authentication if configured
		var pullOpts image.PullOptions
		if c.authManager != nil {
			var err error
			pullOpts, err = c.authManager.GetPullOptions(c.image, platforms.Format(c.platformO))
			if err != nil {
				return "", fmt.Errorf("failed to get pull options: %w", err)
			}
		} else {
			pullOpts = image.PullOptions{Platform: platforms.Format(c.platformO)}
		}

		reader, err := cli.ImagePull(ctx, c.image, pullOpts)
		if err != nil {
			return "", err
		}
		logger.Infof(ctx, "Successfully pulled the image %q", c.image)
		// Output pull-image log to stderr instead of stdout
		_, _ = io.Copy(io.Discard, reader)
	}

	containerConfig := *c.containerConfig
	if len(cmd) > 0 {
		containerConfig.Cmd = cmd
	}
	containerConfig.Image = c.image

	resp, err := cli.ContainerCreate(
		ctx, &containerConfig, c.hostConfig, c.networkConfig, &c.platformO, c.id,
	)
	if err != nil {
		return "", err
	}

	for _, warning := range resp.Warnings {
		logger.Warn(ctx, warning)
	}

	return resp.ID, cli.ContainerStart(ctx, resp.ID, container.StartOptions{})
}

func (c *Client) execInContainer(ctx context.Context, cli *client.Client, cmd []string, stdout, stderr io.Writer, opts ExecOptions) (int, error) {
	// Get container ID from context
	c.mu.Lock()
	containerID := c.id
	c.mu.Unlock()

	// Check if info exists and is running
	info, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return 1, fmt.Errorf("failed to inspect container %s: %w", containerID, err)
	}

	if !info.State.Running {
		return 1, fmt.Errorf("container %s is not running", containerID)
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

	// Override the working dir if specified
	if opts.WorkingDir != "" {
		execOpts.WorkingDir = opts.WorkingDir
	}

	// Create exec instance
	execCreateResp, err := cli.ContainerExecCreate(ctx, containerID, execOpts)
	if err != nil {
		return 1, fmt.Errorf("failed to create exec: %w", err)
	}

	// Start exec instance
	resp, err := cli.ContainerExecAttach(ctx, execCreateResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return 1, fmt.Errorf("failed to start exec: %w", err)
	}
	defer resp.Close()

	// Copy output
	go func() {
		if _, err := stdcopy.StdCopy(stdout, stderr, resp.Reader); err != nil {
			logger.Error(ctx, "docker executor: stdcopy", "err", err)
		}
	}()

	time.Sleep(500 * time.Millisecond) // Give some time for the exec to start

	client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return 1, fmt.Errorf("failed to create docker client: %w", err)
	}

	// Wait for exec to complete
	for {
		inspectResp, err := client.ContainerExecInspect(ctx, execCreateResp.ID)
		if err != nil {
			return 1, fmt.Errorf("failed to inspect exec: %w", err)
		}

		if !inspectResp.Running {
			if inspectResp.ExitCode != 0 {
				return inspectResp.ExitCode, fmt.Errorf("exec failed with exit code: %d", inspectResp.ExitCode)
			}
			return inspectResp.ExitCode, nil
		}

		select {
		case <-ctx.Done():
			return 1, ctx.Err()
		default:
			// Continue waiting
		}
	}
}

func (c *Client) attachAndWait(ctx context.Context, cli *client.Client, containerID string, stdout, stderr io.Writer) (int, error) {
	out, err := cli.ContainerLogs(
		ctx, containerID, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
		},
	)
	if err != nil {
		return 1, err
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
			return 1, err
		}

	case status := <-statusCh:
		if status.StatusCode != 0 {
			return int(status.StatusCode), fmt.Errorf("exit status %v", status.StatusCode)
		}
		return int(status.StatusCode), nil
	}

	// Wait for log copying to complete
	wg.Wait()

	return 0, nil
}

// isDockerInDocker detects if we're running inside a Docker container
func (c *Client) isDockerInDocker() bool {
	// Check multiple indicators of running in a container

	// 1. Check for /.dockerenv file (Docker specific)
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// 2. Check for /run/.containerenv file (Podman specific)
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		return true
	}

	// 3. Check if we're in a container by looking at cgroup
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		// Look for docker, containerd, or other container runtimes in cgroup
		content := string(data)
		if strings.Contains(content, "docker") ||
			strings.Contains(content, "containerd") ||
			strings.Contains(content, "kubepods") ||
			strings.Contains(content, "lxc") {
			return true
		}
	}

	// 4. Check for Kubernetes environment variables
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}

	// 5. Check if Docker socket is mounted (common in docker-in-docker setups)
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		// Additional check: if docker.sock exists AND we have /.dockerenv or container indicators
		// This helps distinguish between a host with Docker installed vs docker-in-docker
		if _, err := os.Stat("/proc/1/cgroup"); err == nil {
			if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
				content := string(data)
				// If cgroup shows we're in a container and docker.sock is mounted, it's docker-in-docker
				if content != "0::/" && content != "" {
					return true
				}
			}
		}
	}

	return false
}

// getPlatform returns the platform of the container
func (c *Client) getPlatform(ctx context.Context, cli *client.Client) (specs.Platform, error) {
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

func (c *Client) shouldPullImage(ctx context.Context, cli *client.Client, platform *specs.Platform) (bool, error) {
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

// NewFromContainerConfig parses digraph.Container into Container struct
func NewFromContainerConfig(workDir string, ct digraph.Container) (*Client, error) {
	return NewFromContainerConfigWithAuth(workDir, ct, nil)
}

// NewFromContainerConfigWithAuth parses digraph.Container into Container struct with registry auth
func NewFromContainerConfigWithAuth(workDir string, ct digraph.Container, registryAuths map[string]*digraph.AuthConfig) (*Client, error) {
	// Validate required fields
	if ct.Image == "" {
		return nil, ErrImageRequired
	}

	// Initialize Docker configuration structs
	containerConfig := &container.Config{
		Image:      ct.Image,
		Env:        ct.Env,
		User:       ct.User,
		WorkingDir: ct.GetWorkingDir(),
	}

	hostConfig := &container.HostConfig{}
	networkConfig := &network.NetworkingConfig{}
	execOptions := &container.ExecOptions{}

	// Parse volumes
	if len(ct.Volumes) > 0 {
		binds, mounts, err := parseVolumes(workDir, ct.Volumes)
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

	client := &Client{
		image:           ct.Image,
		platform:        ct.Platform,
		pull:            ct.PullPolicy,
		autoRemove:      autoRemove,
		containerConfig: containerConfig,
		hostConfig:      hostConfig,
		networkConfig:   networkConfig,
		execOptions:     execOptions,
	}

	// Set up registry authentication if provided
	if len(registryAuths) > 0 {
		client.authManager = NewRegistryAuthManager(registryAuths)
	}

	return client, nil
}

// NewFromMapConfig parses executorConfig into Container struct
func NewFromMapConfig(data map[string]any) (*Client, error) {
	return NewFromMapConfigWithAuth(data, nil)
}

// NewFromMapConfigWithAuth parses executorConfig into Container struct with registry auth
func NewFromMapConfigWithAuth(data map[string]any, registryAuths map[string]*digraph.AuthConfig) (*Client, error) {
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

	client := &Client{
		image:           ret.Image,
		platform:        ret.Platform,
		id:              ret.ContainerName,
		pull:            pull,
		containerConfig: &ret.Container,
		hostConfig:      &ret.Host,
		networkConfig:   &ret.Network,
		execOptions:     &ret.Exec,
		autoRemove:      autoRemove,
	}

	// Set up registry authentication if provided
	if len(registryAuths) > 0 {
		client.authManager = NewRegistryAuthManager(registryAuths)
	}

	return client, nil
}
