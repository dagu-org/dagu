package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/go-viper/mapstructure/v2"
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

var _ Executor = (*docker)(nil)

type docker struct {
	image         string
	containerName string
	pull          bool
	autoRemove    bool
	step          digraph.Step
	stdout        io.Writer
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
	// execConfig is configuration for exec in existing container
	// See https://pkg.go.dev/github.com/docker/docker/api/types/container#ExecOptions
	execConfig *container.ExecOptions
}

func (e *docker) SetStdout(out io.Writer) {
	e.stdout = out
}

func (e *docker) SetStderr(out io.Writer) {
	e.stdout = out
}

func (e *docker) Kill(_ os.Signal) error {
	if e.cancel != nil {
		e.cancel()
	}
	return nil
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
	defer cli.Close()

	// Evaluate args
	var args []string
	for _, arg := range e.step.Args {
		val, err := digraph.EvalString(ctx, arg)
		if err != nil {
			return fmt.Errorf("failed to evaluate arg %s: %w", arg, err)
		}
		args = append(args, val)
	}

	// If containerName is set, use exec instead of creating a new container
	if e.containerName != "" {
		return e.execInContainer(ctx, cli, args)
	}

	// New container creation logic
	if e.pull {
		reader, err := cli.ImagePull(ctx, e.image, image.PullOptions{})
		if err != nil {
			return err
		}
		_, err = io.Copy(e.stdout, reader)
		if err != nil {
			return err
		}
	}

	containerConfig := *e.containerConfig
	containerConfig.Cmd = append([]string{e.step.Command}, args...)
	if e.image != "" {
		containerConfig.Image = e.image
	}

	env := make([]string, len(containerConfig.Env))
	for i, e := range containerConfig.Env {
		env[i], err = digraph.EvalString(ctx, e)
		if err != nil {
			return fmt.Errorf("failed to evaluate env %s: %w", e, err)
		}
	}
	containerConfig.Env = env

	resp, err := cli.ContainerCreate(
		ctx, &containerConfig, e.hostConfig, e.networkConfig, nil, "",
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
	execConfig := container.ExecOptions{
		User:         e.execConfig.User,
		Privileged:   e.execConfig.Privileged,
		Tty:          e.execConfig.Tty,
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          append([]string{e.step.Command}, args...),
		Env:          e.execConfig.Env,
		WorkingDir:   e.execConfig.WorkingDir,
	}

	// Create exec instance
	execID, err := cli.ContainerExecCreate(ctx, e.containerName, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create exec: %w", err)
	}

	// Start exec instance
	resp, err := cli.ContainerExecAttach(ctx, execID.ID, container.ExecAttachOptions{})
	if err != nil {
		return fmt.Errorf("failed to start exec: %w", err)
	}
	defer resp.Close()

	// Copy output
	go func() {
		if _, err := stdcopy.StdCopy(e.stdout, e.stdout, resp.Reader); err != nil {
			logger.Error(ctx, "docker executor: stdcopy", "err", err)
		}
	}()

	// Wait for exec to complete
	for {
		inspectResp, err := cli.ContainerExecInspect(ctx, execID.ID)
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

	go func() {
		if _, err := stdcopy.StdCopy(e.stdout, e.stdout, out); err != nil {
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

	return nil
}

func newDocker(
	ctx context.Context, step digraph.Step,
) (Executor, error) {
	containerConfig := &container.Config{}
	hostConfig := &container.HostConfig{}
	execConfig := &container.ExecOptions{}
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
		replaced, err := digraph.EvalStringFields(ctx, *containerConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate string fields: %w", err)
		}
		containerConfig = &replaced
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
		replaced, err := digraph.EvalStringFields(ctx, *hostConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate string fields: %w", err)
		}
		hostConfig = &replaced
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
		replaced, err := digraph.EvalStringFields(ctx, *networkConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate string fields: %w", err)
		}
		networkConfig = &replaced
	}

	if cfg, ok := execCfg.Config["exec"]; ok {
		md, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			Result: &execConfig,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create decoder: %w", err)
		}
		if err := md.Decode(cfg); err != nil {
			return nil, fmt.Errorf("failed to decode config: %w", err)
		}
		replaced, err := digraph.EvalStringFields(ctx, *execConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate string fields: %w", err)
		}
		execConfig = &replaced
	}

	autoRemove := false
	if hostConfig.AutoRemove {
		hostConfig.AutoRemove = false
		autoRemove = true
	}

	if a, ok := execCfg.Config["autoRemove"]; ok {
		var err error
		autoRemove, err = digraph.EvalBool(ctx, a)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate autoRemove value: %w", err)
		}
	}

	pull := true
	if p, ok := execCfg.Config["pull"]; ok {
		var err error
		pull, err = digraph.EvalBool(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate pull value: %w", err)
		}
	}

	exec := &docker{
		pull:            pull,
		step:            step,
		stdout:          os.Stdout,
		containerConfig: containerConfig,
		hostConfig:      hostConfig,
		networkConfig:   networkConfig,
		execConfig:      execConfig,
		autoRemove:      autoRemove,
	}

	// Check for existing container name first
	if containerName, ok := execCfg.Config["containerName"].(string); ok {
		value, err := digraph.EvalString(ctx, containerName)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate containerName: %w", err)
		}
		exec.containerName = value
		return exec, nil
	}

	// Fall back to image if no container name is provided
	if img, ok := execCfg.Config["image"].(string); ok {
		value, err := digraph.EvalString(ctx, img)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate image: %w", err)
		}
		exec.image = value
		return exec, nil
	}

	return nil, errors.New("either containerName or image must be specified")
}

func init() {
	Register("docker", newDocker)
}
