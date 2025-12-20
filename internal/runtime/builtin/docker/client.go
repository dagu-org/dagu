package docker

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/containerd/errdefs"
	"github.com/containerd/platforms"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/common/signal"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// Errors for container
var (
	ErrImageOrContainerShouldNotBeEmpty = errors.New("containerName or image must be specified")
	ErrImageRequired                    = errors.New("image is required")
	ErrInvalidVolumeFormat              = errors.New("invalid volume format")
	ErrInvalidPortFormat                = errors.New("invalid port format")
	ErrContainerIsNotRunning            = errors.New("container is not running")
	// Validation errors for docker executor map config
	ErrExecOnlyWithContainerName       = errors.New("'exec' options require 'containerName' (exec-in-existing mode)")
	ErrInvalidOptionsWithContainerName = errors.New("'container', 'host', 'network', 'pull', 'platform', or 'autoRemove' not supported with 'containerName'")
)

// Constants for container operations
const (
	// errorExitCode is the exit code to return when an error occurs and we
	// cannot get a more specific code
	errorExitCode = 1

	// Default timeout values
	defaultReadinessTimeout = 120 * time.Second
	defaultPollInterval     = 500 * time.Millisecond

	// Container runtime detection files
	dockerEnvFile    = "/.dockerenv"
	podmanEnvFile    = "/run/.containerenv"
	proc1CgroupFile  = "/proc/1/cgroup"
	dockerSocketFile = "/var/run/docker.sock"

	// Keepalive settings
	keepAliveSleepCmd   = "while true; do sleep 86400; done"
	keepAliveTargetPath = "/__dagu_runner/keepalive"

	// Log scanning buffer sizes
	logScanInitialBuf = 64 * 1024
	logScanMaxBuf     = 1024 * 1024
)

type Client struct {
	cfg *Config

	platform    specs.Platform // resolved platform
	containerID string         // ID of the running container (if any)
	started     atomic.Bool

	mu  sync.Mutex
	cli *client.Client

	keepAliveTmp string

	// authManager handles registry authentication
	authManager *RegistryAuthManager

	cancelMu sync.Mutex
	cancel   func()
}

// ExecOptions specifies options to execute commands in the container.
type ExecOptions struct {
	// WorkingDir overrides the working directory for the exec command.
	WorkingDir string
}

// InitializeClient creates a new container client
func InitializeClient(ctx context.Context, cfg *Config) (*Client, error) {
	dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	platform, err := getPlatform(ctx, dockerCli, cfg)
	if err != nil {
		return nil, err
	}

	// Check if the container is running when containerName is specified
	var ctID string
	var name = strings.TrimSpace(cfg.ContainerName)
	if name != "" {
		info, inspectErr := dockerCli.ContainerInspect(ctx, name)
		isContainerRunning, err := isContainerRunning(info, inspectErr)
		if err != nil {
			return nil, fmt.Errorf("failed to check if container %q is running: %w", name, err)
		}
		if info.ContainerJSONBase != nil {
			ctID = info.ID
		}
		if cfg.Image == "" && !isContainerRunning {
			return nil, fmt.Errorf("container %q is not running", name)
		}
	}

	return &Client{
		cfg:         cfg,
		containerID: ctID,
		cli:         dockerCli,
		platform:    platform,
		authManager: cfg.AuthManager,
	}, nil
}

// Close closes the container client and cleans up resources
func (c *Client) Close(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If we have a running container and autoRemove is set, remove it
	if c.cfg.AutoRemove && c.started.Load() && c.containerID != "" {
		if err := c.cli.ContainerRemove(context.Background(), c.containerID, container.RemoveOptions{Force: true}); err != nil {
			logger.Error(ctx, "Docker executor: remove container", tag.Error(err))
		}
	}

	_ = c.cli.Close()
	c.cli = nil
}

// Exec executes the command in the running container
func (c *Client) Exec(ctx context.Context, cmd []string, stdout, stderr io.Writer, opts ExecOptions) (int, error) {
	c.mu.Lock()
	if c.containerID == "" {
		c.mu.Unlock()
		return 1, ErrContainerIsNotRunning
	}
	cli := c.cli
	c.mu.Unlock()

	return c.execInContainer(ctx, cli, cmd, stdout, stderr, opts)
}

// CreateContainerKeepAlive creates the container that lives while the DAG running
func (c *Client) CreateContainerKeepAlive(ctx context.Context) error {
	if c.containerID != "" {
		return fmt.Errorf("container already exists. id=%s", c.containerID)
	}

	// Check if a container with the specified name already exists
	if name := c.cfg.ContainerName; name != "" {
		info, err := c.cli.ContainerInspect(ctx, name)
		if err == nil {
			// Container exists - fail regardless of state
			if info.State != nil && info.State.Running {
				return fmt.Errorf("container with name %q already exists and is running", name)
			}
			return fmt.Errorf("container with name %q already exists", name)
		}
		// If error is not "not found", it's an unexpected error
		if !errdefs.IsNotFound(err) {
			return fmt.Errorf("failed to check existing container %q: %w", name, err)
		}
		// Container doesn't exist, proceed to create
	}

	// Choose startup mode and command
	var cmd []string
	mode := c.cfg.Startup
	if mode == "" {
		mode = "keepalive"
	}

	switch mode {
	case "keepalive":
		if len(c.cfg.Container.Cmd) == 0 {
			// Detect if we're running in docker-in-docker environment
			isDockerInDocker := c.isDockerInDocker()

			if isDockerInDocker {
				logger.Info(ctx, "Detected docker-in-docker environment, using sleep for keepalive")
				cmd = []string{"sh", "-c", keepAliveSleepCmd}
			} else {
				cmd = c.setupKeepaliveCommand(ctx)
			}
		}
	case "entrypoint":
		// Respect image ENTRYPOINT/CMD: do not set cmd; run as-is
		cmd = nil
	case "command":
		if len(c.cfg.StartCmd) == 0 {
			return fmt.Errorf("startup 'command' requires non-empty command array")
		}
		cmd = append([]string{}, c.cfg.StartCmd...)
	default:
		return fmt.Errorf("invalid startup mode: %s", mode)
	}

	// Set init true to prevent zombie subprocess issues
	init := true
	c.cfg.Host.Init = &init

	ctx, cancel := context.WithCancel(ctx)
	c.cancelMu.Lock()
	c.cancel = cancel
	c.cancelMu.Unlock()

	ctID, err := c.startNewContainer(ctx, c.cfg.ContainerName, c.cli, cmd, true)
	if err != nil {
		return fmt.Errorf("failed to start a new container: %w", err)
	}
	c.containerID = ctID

	// Readiness wait
	waitMode := c.cfg.WaitFor
	if waitMode == "" {
		waitMode = "running"
	}
	// Default timeout for readiness
	readyCtx, cancel := context.WithTimeout(ctx, defaultReadinessTimeout)
	defer cancel()

	switch waitMode {
	case "running":
		if err := c.waitRunning(readyCtx, c.cli, ctID); err != nil {
			return err
		}
	case "healthy":
		// If no healthcheck defined, warn and fallback to running
		hasHealth, err := c.hasHealthcheck(readyCtx, c.cli, ctID)
		if err != nil {
			return err
		}
		if !hasHealth {
			logger.Warn(ctx, "Selected waitFor=healthy but image has no healthcheck; falling back to running")
			if err := c.waitRunning(readyCtx, c.cli, ctID); err != nil {
				return err
			}
		} else {
			if err := c.waitHealthy(readyCtx, c.cli, ctID); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("invalid waitFor mode: %s", waitMode)
	}

	// Optional log pattern wait after base readiness
	if strings.TrimSpace(c.cfg.LogPattern) != "" {
		if err := c.waitLogPattern(readyCtx, c.cli, ctID, c.cfg.LogPattern); err != nil {
			return err
		}
	}

	return nil
}

// setupKeepaliveCommand configures the keepalive command for non-docker-in-docker environments
func (c *Client) setupKeepaliveCommand(ctx context.Context) []string {
	// Standard environment, use the keepalive binary
	hostPath, err := GetKeepaliveFile(c.platform)
	if err != nil {
		// Fallback to sleep if keepalive binary fails
		logger.Warn(ctx, "Failed to get keepalive binary; using sleep fallback", tag.Error(err))
		return []string{"sh", "-c", keepAliveSleepCmd}
	}

	c.keepAliveTmp = hostPath
	// Setup the volume bind for the keepalive binary
	bind := hostPath + ":" + keepAliveTargetPath + ":ro"
	c.cfg.Host.Binds = append(c.cfg.Host.Binds, bind)
	return []string{keepAliveTargetPath}
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

	if c.containerID != "" {
		// Stop the container
		if err := c.cli.ContainerStop(ctx, c.containerID, container.StopOptions{}); err != nil {
			logger.Error(ctx, "Docker executor: stop container", tag.Error(err))
		}
	}

	if c.containerID != "" {
		// Forcefully stop after timeout
		defaultStopTimeout := 5 * time.Second
		var containerStopped atomic.Bool
		forceKillTimer := time.AfterFunc(defaultStopTimeout, func() {
			if containerStopped.Load() {
				return
			}
			if err := c.cli.ContainerKill(context.Background(), c.containerID, "SIGKILL"); err != nil {
				if errdefs.IsNotFound(err) {
					return
				}
				logger.Error(ctx, "Docker executor: force stop container", tag.Error(err))
			}
		})

		// Wait for container to be fully stopped
		maxWait := 30 * time.Second
		waitTimer := time.NewTimer(maxWait)
		defer waitTimer.Stop()

	WAIT_LOOP:
		for {
			info, err := c.cli.ContainerInspect(context.Background(), c.containerID)
			if err != nil {
				if errdefs.IsNotFound(err) {
					containerStopped.Store(true)
					break
				}
				logger.Error(ctx, "Docker executor: inspect container",
					tag.Error(err),
				)
			} else if info.State != nil && !info.State.Running {
				containerStopped.Store(true)
				break
			}

			select {
			case <-waitTimer.C:
				logger.Warn(ctx, "Docker executor: timeout waiting for container to stop")
				break WAIT_LOOP
			default:
				time.Sleep(defaultPollInterval)
			}
		}

		if containerStopped.Load() {
			_ = forceKillTimer.Stop()
		}
	}

	if c.keepAliveTmp != "" {
		// Remove the temporary keep alive file if it exists
		if err := os.Remove(c.keepAliveTmp); err != nil && !os.IsNotExist(err) {
			logger.Error(ctx, "Docker executor: remove keep alive file", tag.Error(err))
		}
	}
	c.keepAliveTmp = ""
}

// Run executes the command in the container and returns exit code
func (c *Client) Run(ctx context.Context, cmd []string, stdout, stderr io.Writer) (int, error) {
	ctID := c.containerID

	// check if container with the same name already exists
	if ctID != "" {
		// Check if the container is running
		info, err := c.cli.ContainerInspect(ctx, ctID)
		if err != nil && !errdefs.IsNotFound(err) {
			return errorExitCode, fmt.Errorf("failed to inspect container %s: %w", ctID, err)
		}
		// Container exists and is running; exec in it
		if err == nil && info.State != nil && info.State.Running {
			return c.execInContainer(ctx, c.cli, cmd, stdout, stderr, ExecOptions{})
		}
		// If shouldStart is false, return error
		if !c.cfg.ShouldStart {
			return errorExitCode, fmt.Errorf("container %s already exists and is not running", ctID)
		}
	}

	// If container is not running, start a new one
	// The container should be stopped and removed after run with autoRemove
	// set to true.
	ctID, err := c.startNewContainer(ctx, c.cfg.ContainerName, c.cli, cmd, false)
	if err != nil {
		return errorExitCode, fmt.Errorf("failed to start a new container: %w", err)
	}

	var once sync.Once
	defer func() {
		if !c.cfg.AutoRemove {
			return
		}

		once.Do(func() {
			if err := c.cli.ContainerRemove(context.Background(), c.containerID, container.RemoveOptions{Force: true}); err != nil {
				logger.Error(ctx, "Docker executor: remove container", tag.Error(err))
			}
		})
	}()

	exitCode, err := c.attachAndWait(ctx, c.cli, ctID, stdout, stderr)

	// Wait for container to be stopped before returning
	for {
		info, err := c.cli.ContainerInspect(context.Background(), ctID)
		if err != nil {
			if errdefs.IsNotFound(err) {
				break
			}
			return exitCode, fmt.Errorf("failed to inspect container %s: %w", ctID, err)
		}
		if info.State != nil && !info.State.Running {
			break
		}

		time.Sleep(defaultPollInterval)
	}

	return exitCode, err
}

// Stop stops the running container
func (c *Client) Stop(sig os.Signal) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.containerID == "" {
		return nil
	}

	info, err := c.cli.ContainerInspect(context.Background(), c.containerID)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to inspect container %s: %w", c.containerID, err)
	}
	if info.State != nil && !info.State.Running {
		return nil
	}

	var sigName string
	if sysSig, ok := sig.(syscall.Signal); ok {
		sigName = signal.GetSignalName(sysSig)
	}

	if err := c.cli.ContainerStop(context.Background(), c.containerID, container.StopOptions{Signal: sigName}); err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return err
	}

	return nil
}

func (c *Client) startNewContainer(ctx context.Context, name string, cli *client.Client, cmd []string, clearEntrypoint bool) (string, error) {
	pull, err := c.shouldPullImage(ctx, cli, &c.platform)
	if err != nil {
		return "", err
	}

	logger.Info(ctx, "Creating a new container",
		slog.Any("platform", c.platform),
		slog.String("image", c.cfg.Container.Image),
		slog.String("pull-policy", c.cfg.Pull.String()),
		slog.Bool("should-pull", pull),
	)

	if pull {
		logger.Infof(ctx, "Pulling the image %q", c.cfg.Image)

		// Get pull options with authentication if configured
		var pullOpts image.PullOptions
		if c.authManager != nil {
			var err error
			pullOpts, err = c.authManager.GetPullOptions(c.cfg.Image, platforms.Format(c.platform))
			if err != nil {
				return "", fmt.Errorf("failed to get pull options: %w", err)
			}
		} else {
			pullOpts = image.PullOptions{Platform: platforms.Format(c.platform)}
		}

		reader, err := cli.ImagePull(ctx, c.cfg.Image, pullOpts)
		if err != nil {
			return "", err
		}
		logger.Infof(ctx, "Successfully pulled the image %q", c.cfg.Image)
		// Output pull-image log to stderr instead of stdout
		_, _ = io.Copy(io.Discard, reader)
	}

	ctCfg := *c.cfg.Container // Copy to avoid mutating original
	ctCfg.Image = c.cfg.Image

	if len(cmd) > 0 {
		ctCfg.Cmd = cmd
		if clearEntrypoint {
			// Entrypoint should be empty slice to override image ENTRYPOINT
			ctCfg.Entrypoint = []string{}
		}
	}

	resp, err := cli.ContainerCreate(
		ctx, &ctCfg, c.cfg.Host, c.cfg.Network, &c.platform, name,
	)
	if err != nil {
		return "", err
	}

	for _, warning := range resp.Warnings {
		logger.Warn(ctx, warning)
	}

	c.containerID = resp.ID

	err = cli.ContainerStart(ctx, resp.ID, container.StartOptions{})

	if err == nil {
		c.started.Store(true)
	}

	return resp.ID, err
}

func (c *Client) execInContainer(ctx context.Context, cli *client.Client, cmd []string, stdout, stderr io.Writer, opts ExecOptions) (int, error) {
	// Get container ID from context
	c.mu.Lock()
	containerID := c.containerID
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
		User:         c.cfg.ExecOptions.User,
		Privileged:   c.cfg.ExecOptions.Privileged,
		Tty:          c.cfg.ExecOptions.Tty,
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
		Env:          c.cfg.ExecOptions.Env,
		WorkingDir:   c.cfg.ExecOptions.WorkingDir,
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
	var wg sync.WaitGroup
	wg.Add(1)
	defer wg.Wait()

	go func() {
		if _, err := stdcopy.StdCopy(stdout, stderr, resp.Reader); err != nil {
			logger.Error(ctx, "Docker executor: stdcopy", tag.Error(err))
		}
		wg.Done()
	}()

	time.Sleep(defaultPollInterval) // Give some time for the exec to start

	// Wait for exec to complete
	for {
		inspectResp, err := cli.ContainerExecInspect(ctx, execCreateResp.ID)
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
			time.Sleep(defaultPollInterval)
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
			logger.Error(ctx, "Docker executor: stdcopy", tag.Error(err))
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

	// 1. Check for Docker environment file
	if c.fileExists(dockerEnvFile) {
		return true
	}

	// 2. Check for Podman environment file
	if c.fileExists(podmanEnvFile) {
		return true
	}

	// 3. Check if we're in a container by examining cgroup
	if c.isInContainerByCgroup() {
		return true
	}

	// 4. Check for Kubernetes environment
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}

	// 5. Check if Docker socket is mounted (docker-in-docker scenario)
	if c.fileExists(dockerSocketFile) && c.isInContainerByCgroup() {
		return true
	}

	return false
}

// fileExists checks if a file exists
func (c *Client) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// isInContainerByCgroup checks if we're running in a container by examining cgroup
func (c *Client) isInContainerByCgroup() bool {
	data, err := os.ReadFile(proc1CgroupFile)
	if err != nil {
		return false
	}

	content := string(data)
	// Look for container runtime indicators in cgroup
	containerIndicators := []string{"docker", "containerd", "kubepods", "lxc"}
	for _, indicator := range containerIndicators {
		if strings.Contains(content, indicator) {
			return true
		}
	}

	// Additional check for non-root cgroup (indicates containerization)
	return content != "0::/" && content != ""
}

func getPlatform(ctx context.Context, cli *client.Client, cfg *Config) (specs.Platform, error) {
	// Extract platform from the current input and fallback to the current docker host platform.
	var platform specs.Platform
	if cfg.Platform != "" {
		var err error
		platform, err = platforms.Parse(cfg.Platform)
		if err != nil {
			return platform, fmt.Errorf("failed to parse platform %s: %w", cfg.Platform, err)
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
	if c.cfg.Pull == core.PullPolicyAlways {
		return true, nil
	}
	if c.cfg.Pull == core.PullPolicyNever {
		return false, nil
	}

	// Loop through all locally available images that have the same reference with
	// the input image to check if we have the correct platform.
	filters := filters.NewArgs()
	filters.Add("reference", c.cfg.Image)

	images, err := cli.ImageList(ctx, image.ListOptions{Filters: filters})
	if err != nil {
		return false, fmt.Errorf("failed to list local images %s: %w", c.cfg.Image, err)
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

// parseRestartPolicy parses a docker restart policy string into container.RestartPolicy.
// Supported forms: "no", "always", "unless-stopped" (on-failure not supported).
func parseRestartPolicy(s string) (container.RestartPolicy, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return container.RestartPolicy{}, nil
	}
	switch s { // use tagged switch to satisfy linter
	case "no":
		return container.RestartPolicy{Name: "no"}, nil
	case "always":
		return container.RestartPolicy{Name: "always"}, nil
	case "unless-stopped":
		return container.RestartPolicy{Name: "unless-stopped"}, nil
	default:
		return container.RestartPolicy{}, fmt.Errorf("invalid restartPolicy: %s (supported: no, always, unless-stopped)", s)
	}
}

// waitRunning waits until the container is in running state or context times out.
func (c *Client) waitRunning(ctx context.Context, cli *client.Client, id string) error {
	ticker := time.NewTicker(defaultPollInterval)
	defer ticker.Stop()
	var last string
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("readiness timeout waiting for running; last state=%s: %w", last, ctx.Err())
		case <-ticker.C:
			info, err := cli.ContainerInspect(ctx, id)
			if err != nil {
				return fmt.Errorf("failed to inspect container %s: %w", id, err)
			}
			if info.State != nil {
				if info.State.Running {
					logger.Info(ctx, "Container ready (running)",
						slog.String("id", id),
					)
					return nil
				}
				// If the container has already exited or is dead, fail fast
				if status := strings.ToLower(info.State.Status); status == "exited" || status == "dead" || status == "removing" { //nolint:gocritic
					return fmt.Errorf("container %s not running; status=%s, exitCode=%d", id, status, info.State.ExitCode)
				}
				last = fmt.Sprintf("running=%v,status=%s", info.State.Running, info.State.Status)
			}
		}
	}
}

// hasHealthcheck checks if the container has a healthcheck configured.
func (c *Client) hasHealthcheck(ctx context.Context, cli *client.Client, id string) (bool, error) {
	info, err := cli.ContainerInspect(ctx, id)
	if err != nil {
		return false, fmt.Errorf("failed to inspect container %s: %w", id, err)
	}
	return info.State != nil && info.State.Health != nil, nil
}

// waitHealthy waits until the container health status is healthy.
func (c *Client) waitHealthy(ctx context.Context, cli *client.Client, id string) error {
	ticker := time.NewTicker(defaultPollInterval)
	defer ticker.Stop()
	var last string
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("readiness timeout waiting for healthy; last health=%s: %w", last, ctx.Err())
		case <-ticker.C:
			info, err := cli.ContainerInspect(ctx, id)
			if err != nil {
				return fmt.Errorf("failed to inspect container %s: %w", id, err)
			}
			if info.State != nil && info.State.Health != nil {
				status := info.State.Health.Status
				last = status
				if strings.ToLower(status) == "healthy" {
					logger.Info(ctx, "Container ready (healthy)",
						slog.String("id", id),
					)
					return nil
				}
			}
		}
	}
}

// waitLogPattern follows container logs until the given regex pattern appears.
func (c *Client) waitLogPattern(ctx context.Context, cli *client.Client, id string, pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid logPattern regex: %w", err)
	}
	reader, err := cli.ContainerLogs(ctx, id, container.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: true, Tail: "all"})
	if err != nil {
		return fmt.Errorf("failed to read container logs: %w", err)
	}
	defer func() {
		if cerr := reader.Close(); cerr != nil {
			logger.Error(ctx, "Docker executor: close logs reader", tag.Error(cerr))
		}
	}()

	pr, pw := io.Pipe()
	// Demultiplex logs into a single stream
	go func() {
		defer func() {
			if cerr := pw.Close(); cerr != nil {
				logger.Error(ctx, "Docker executor: close pipe writer", tag.Error(cerr))
			}
		}()
		_, _ = stdcopy.StdCopy(pw, pw, reader)
	}()

	scanner := bufio.NewScanner(pr)
	// Allow long lines for log parsing
	buf := make([]byte, 0, logScanInitialBuf)
	scanner.Buffer(buf, logScanMaxBuf)
	for scanner.Scan() {
		line := scanner.Text()
		if re.MatchString(line) {
			logger.Info(ctx, "Container ready (log pattern matched)",
				slog.String("id", id),
				slog.String("pattern", pattern),
			)
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("readiness timeout waiting for logPattern; pattern=%q: %w", pattern, ctx.Err())
		default:
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading logs: %w", err)
	}
	return fmt.Errorf("log stream ended before pattern matched: %q", pattern)
}

func isContainerRunning(info container.InspectResponse, err error) (bool, error) {
	if err != nil {
		if errdefs.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return (info.State != nil && info.State.Running), nil
}
