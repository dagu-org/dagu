package container

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/containerd/platforms"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
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
)

type Client struct {
	image      string
	platform   string
	nameOrID   string
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
	execOptions *container.ExecOptions
	mu          sync.Mutex
	client      *client.Client
}

// Open creates a new client
func (c *Client) Open() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	c.client = cli
	return nil
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client == nil {
		return
	}
	_ = c.client.Close()
	c.client = nil
}

// Run executes the command in the container and returns exit code
func (c *Client) Run(ctx context.Context, cmd []string, stdout, stderr io.Writer) (int, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return 1, err
	}

	defer func() {
		_ = cli.Close()
	}()

	c.mu.Lock()
	if c.nameOrID != "" {
		c.mu.Unlock()
		return c.execInContainer(ctx, cli, cmd, stdout, stderr)
	}

	id, err := c.startNewContainer(ctx, cli, cmd)
	c.nameOrID = id
	c.mu.Unlock()

	var once sync.Once
	defer func() {
		if !c.autoRemove {
			return
		}

		once.Do(func() {
			c.mu.Lock()
			id := c.nameOrID
			c.mu.Unlock()

			if err := cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}); err != nil {
				logger.Error(ctx, "docker executor: remove container", "err", err)
			}
		})
	}()

	return c.attachAndWait(ctx, cli, id, stdout, stderr)
}

func (c *Client) startNewContainer(ctx context.Context, cli *client.Client, cmd []string) (string, error) {
	platform, err := c.getPlatform(ctx, cli)
	if err != nil {
		return "", err
	}

	pull, err := c.shouldPullImage(ctx, cli, &platform)
	if err != nil {
		return "", err
	}

	logger.Info(ctx, "Creating a new container", "platform", platform, "image", c.image)

	if pull {
		logger.Infof(ctx, "Pulling the image %q", c.image)
		reader, err := cli.ImagePull(ctx, c.image, image.PullOptions{Platform: platforms.Format(platform)})
		if err != nil {
			return "", err
		}
		logger.Infof(ctx, "Successfully pulled the image %q", c.image)
		// Output pull-image log to stderr instead of stdout
		_, _ = io.Copy(io.Discard, reader)
	}

	containerConfig := *c.containerConfig
	containerConfig.Cmd = cmd
	containerConfig.Image = c.image

	resp, err := cli.ContainerCreate(
		ctx, &containerConfig, c.hostConfig, c.networkConfig, &platform, c.nameOrID,
	)
	if err != nil {
		return "", err
	}

	for _, warning := range resp.Warnings {
		logger.Warn(ctx, warning)
	}

	return resp.ID, cli.ContainerStart(ctx, resp.ID, container.StartOptions{})
}

func (c *Client) execInContainer(ctx context.Context, cli *client.Client, cmd []string, stdout, stderr io.Writer) (int, error) {
	// Check if info exists and is running
	info, err := cli.ContainerInspect(ctx, c.nameOrID)
	if err != nil {
		return 1, fmt.Errorf("failed to inspect container %s: %w", c.nameOrID, err)
	}

	if !info.State.Running {
		return 1, fmt.Errorf("container %s is not running", c.nameOrID)
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
	containerID, err := cli.ContainerExecCreate(ctx, c.nameOrID, execOpts)
	if err != nil {
		return 1, fmt.Errorf("failed to create exec: %w", err)
	}

	// Start exec instance
	resp, err := cli.ContainerExecAttach(ctx, containerID.ID, container.ExecAttachOptions{})
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

	// Wait for exec to complete
	for {
		inspectResp, err := cli.ContainerExecInspect(ctx, containerID.ID)
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
