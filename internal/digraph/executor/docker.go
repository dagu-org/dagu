package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"

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
	"github.com/pkg/errors"
)

// Docker executor runs a command in a Docker container.
/* Example DAG:
```yaml
steps:
 - name: exec-in-existing
   executor:
     type: docker
     config:
       containerName: <container-name>
       autoRemove: true
       exec:
         user: root     # optional
         workingDir: /  # optional
         env:           # optional
           - MY_VAR=value
   command: echo "Hello from existing container"

 - name: create-new
   executor:
     type: docker
     config:
       image: alpine:latest
       autoRemove: true
   command: echo "Hello from new container"
```
*/

type PullPolicy int

const (
	PullPolicyAlways PullPolicy = iota
	PullPolicyNever
	PullPolicyMissing
)

var pullPolicyMap = map[string]PullPolicy{
	"always":  PullPolicyAlways,
	"missing": PullPolicyMissing,
	"never":   PullPolicyNever,
}

func boolToPullPolicy(b bool) PullPolicy {
	if b {
		return PullPolicyAlways
	}
	return PullPolicyNever
}

func parsePullPolicy(_ context.Context, raw any) (PullPolicy, error) {
	switch value := raw.(type) {
	case string:
		// Try to parse the string as a pull policy
		pull, ok := pullPolicyMap[value]
		if ok {
			return pull, nil
		}

		// If the string is not a valid pull policy, try to parse it as a boolean
		b, err := strconv.ParseBool(value)
		if err != nil {
			return PullPolicyMissing, fmt.Errorf("failed to parse pull policy as boolean: %w", err)
		}
		return boolToPullPolicy(b), nil
	case bool:
		return boolToPullPolicy(value), nil
	default:
		return PullPolicyMissing, fmt.Errorf("invalid pull policy type: %T", raw)
	}
}

var _ Executor = (*docker)(nil)

type docker struct {
	image         string
	platform      string
	containerName string
	pull          PullPolicy
	autoRemove    bool
	step          digraph.Step
	stdout        io.Writer
	stderr        io.Writer
	context       context.Context
	cancel        func()
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

func (e *docker) SetStdout(out io.Writer) {
	e.stdout = out
}

func (e *docker) SetStderr(out io.Writer) {
	e.stderr = out
}

func (e *docker) Kill(_ os.Signal) error {
	if e.cancel != nil {
		e.cancel()
	}
	return nil
}

func (e *docker) getPlatform(ctx context.Context, cli *client.Client) (specs.Platform, error) {
	// Extract platform from the current input and fallback to the current docker host platform.
	var platform specs.Platform
	if e.platform != "" {
		var err error
		platform, err = platforms.Parse(e.platform)
		if err != nil {
			return platform, fmt.Errorf("failed to parse platform %s: %w", e.platform, err)
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

func (e *docker) shouldPullImage(ctx context.Context, cli *client.Client, platform *specs.Platform) (bool, error) {
	if e.pull == PullPolicyAlways {
		return true, nil
	}
	if e.pull == PullPolicyNever {
		return false, nil
	}

	// Loop through all locally available images that have the same reference with
	// the input image to check if we have the correct platform.
	filters := filters.NewArgs()
	filters.Add("reference", e.image)

	images, err := cli.ImageList(ctx, image.ListOptions{Filters: filters})
	if err != nil {
		return false, fmt.Errorf("failed to list local images %s: %w", e.image, err)
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

func (e *docker) Run(ctx context.Context) error {
	ctx, cancelFunc := context.WithCancel(ctx)
	e.context = ctx
	e.cancel = cancelFunc

	cli, err := client.NewClientWithOpts(
		client.FromEnv, client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return err
	}
	defer func() {
		_ = cli.Close()
	}()

	if e.image == "" {
		return e.execInContainer(ctx, cli, e.step.Args)
	}

	platform, err := e.getPlatform(ctx, cli)
	if err != nil {
		return err
	}

	pull, err := e.shouldPullImage(ctx, cli, &platform)
	if err != nil {
		return err
	}
	if pull {
		reader, err := cli.ImagePull(ctx, e.image, image.PullOptions{Platform: platforms.Format(platform)})
		if err != nil {
			return err
		}
		_, err = io.Copy(e.stdout, reader)
		if err != nil {
			return err
		}
	}

	containerConfig := *e.containerConfig
	containerConfig.Cmd = append([]string{e.step.Command}, e.step.Args...)
	containerConfig.Image = e.image

	resp, err := cli.ContainerCreate(
		ctx, &containerConfig, e.hostConfig, e.networkConfig, &platform, e.containerName,
	)
	if err != nil {
		return err
	}

	var once sync.Once
	removeContainer := func() {
		if !e.autoRemove {
			return
		}
		once.Do(func() {
			if err := cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true}); err != nil {
				logger.Error(ctx, "docker executor: remove container", "err", err)
			}
		})
	}

	defer removeContainer()
	e.cancel = func() {
		removeContainer()
		cancelFunc()
	}

	if err := cli.ContainerStart(
		ctx, resp.ID, container.StartOptions{},
	); err != nil {
		return err
	}

	return e.attachAndWait(ctx, cli, resp.ID)
}

func (e *docker) execInContainer(ctx context.Context, cli *client.Client, args []string) error {
	// Check if containerInfo exists and is running
	containerInfo, err := cli.ContainerInspect(ctx, e.containerName)
	if err != nil {
		return fmt.Errorf("failed to inspect container %s: %w", e.containerName, err)
	}

	if !containerInfo.State.Running {
		return fmt.Errorf("container %s is not running", e.containerName)
	}

	// Create exec configuration
	execOpts := container.ExecOptions{
		User:         e.execOptions.User,
		Privileged:   e.execOptions.Privileged,
		Tty:          e.execOptions.Tty,
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          append([]string{e.step.Command}, args...),
		Env:          e.execOptions.Env,
		WorkingDir:   e.execOptions.WorkingDir,
	}

	// Create exec instance
	containerID, err := cli.ContainerExecCreate(ctx, e.containerName, execOpts)
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
		if _, err := stdcopy.StdCopy(e.stdout, e.stderr, resp.Reader); err != nil {
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

func (e *docker) attachAndWait(ctx context.Context, cli *client.Client, containerID string) error {
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
	defer out.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := stdcopy.StdCopy(e.stdout, e.stderr, out); err != nil {
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

func newDocker(
	ctx context.Context, step digraph.Step,
) (Executor, error) {
	containerConfig := &container.Config{}
	hostConfig := &container.HostConfig{}
	execOpts := &container.ExecOptions{}
	networkConfig := &network.NetworkingConfig{}

	execCfg := step.ExecutorConfig

	if cfg, ok := execCfg.Config["container"]; ok {
		md, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			Result:           containerConfig,
			WeaklyTypedInput: true,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create decoder: %w", err)
		}
		if err := md.Decode(cfg); err != nil {
			return nil, fmt.Errorf("failed to decode config: %w", err)
		}
	}

	if cfg, ok := execCfg.Config["host"]; ok {
		md, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			Result: hostConfig,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create decoder: %w", err)
		}
		if err := md.Decode(cfg); err != nil {
			return nil, fmt.Errorf("failed to decode config: %w", err)
		}
	}

	if cfg, ok := execCfg.Config["network"]; ok {
		md, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			Result: networkConfig,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create decoder: %w", err)
		}
		if err := md.Decode(cfg); err != nil {
			return nil, fmt.Errorf("failed to decode config: %w", err)
		}
	}

	if cfg, ok := execCfg.Config["exec"]; ok {
		md, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			Result: &execOpts,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create decoder: %w", err)
		}
		if err := md.Decode(cfg); err != nil {
			return nil, fmt.Errorf("failed to decode config: %w", err)
		}
	}

	autoRemove := false
	if hostConfig.AutoRemove {
		hostConfig.AutoRemove = false
		autoRemove = true
	}

	if a, ok := execCfg.Config["autoRemove"]; ok {
		var err error
		autoRemove, err = stringutil.ParseBool(ctx, a)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate autoRemove value: %w", err)
		}
	}

	pull := PullPolicyMissing
	if raw, ok := execCfg.Config["pull"]; ok {
		var err error
		pull, err = parsePullPolicy(ctx, raw)
		if err != nil {
			return nil, err
		}
	}

	platform := ""
	if value, ok := execCfg.Config["platform"].(string); ok {
		platform = value
	}

	containerName := ""
	if value, ok := execCfg.Config["containerName"].(string); ok {
		containerName = value
	}

	exec := &docker{
		platform:        platform,
		containerName:   containerName,
		pull:            pull,
		step:            step,
		stdout:          os.Stdout,
		stderr:          os.Stderr,
		containerConfig: containerConfig,
		hostConfig:      hostConfig,
		networkConfig:   networkConfig,
		execOptions:     execOpts,
		autoRemove:      autoRemove,
	}

	// If image is provided, we don't care about containerName and will create a new container
	if img, ok := execCfg.Config["image"].(string); ok {
		exec.image = img
		return exec, nil
	}

	// If image is not provided, containerName must be provided so we can use it in exec mode
	if exec.containerName == "" {
		return nil, errors.New("at least containerName or image must be specified")
	} else {
		return exec, nil
	}
}

func init() {
	Register("docker", newDocker)
}
