package executor

// See https://docs.docker.com/engine/api/sdk/

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/utils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/mitchellh/mapstructure"
)

type DockerExecutor struct {
	image           string
	autoRemove      bool
	step            *dag.Step
	containerConfig *container.Config
	hostConfig      *container.HostConfig
	stdout          io.Writer
	context         context.Context
	cancel          func()
}

func (e *DockerExecutor) SetStdout(out io.Writer) {
	e.stdout = out
}

func (e *DockerExecutor) SetStderr(out io.Writer) {
	e.stdout = out
}

func (e *DockerExecutor) Kill(sig os.Signal) error {
	if e.cancel != nil {
		e.cancel()
	}
	return nil
}

func (e *DockerExecutor) Run() error {
	ctx, fn := context.WithCancel(context.Background())
	e.context = ctx
	e.cancel = fn

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()

	reader, err := cli.ImagePull(ctx, e.image, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	_, err = io.Copy(e.stdout, reader)
	if err != nil {
		return err
	}

	if e.image != "" {
		e.containerConfig.Image = e.image
	}
	e.containerConfig.Cmd = append([]string{e.step.Command}, e.step.Args...)

	resp, err := cli.ContainerCreate(ctx, e.containerConfig, e.hostConfig, nil, nil, "")

	if err != nil {
		return err
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case <-statusCh:
	}

	out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		return err
	}

	_, err = stdcopy.StdCopy(e.stdout, e.stdout, out)
	utils.LogErr("docker executor: stdcopy", err)

	if e.autoRemove {
		err := cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{})
		utils.LogErr("docker executor: remove container", err)
	}

	return nil
}

func CreateDockerExecutor(ctx context.Context, step *dag.Step) (Executor, error) {
	containerConfig := &container.Config{}
	hostConfig := &container.HostConfig{}
	execCfg := step.ExecutorConfig

	if cfg, ok := execCfg.Config["container"]; ok {
		// See https://pkg.go.dev/github.com/docker/docker/api/types/container#Config
		md, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			Result: containerConfig,
		})

		if err != nil {
			return nil, fmt.Errorf("failed to create decoder: %w", err)
		}

		if err := md.Decode(cfg); err != nil {
			return nil, fmt.Errorf("failed to decode config: %w", err)
		}
	}

	if cfg, ok := execCfg.Config["host"]; ok {
		// See https://pkg.go.dev/github.com/docker/docker/api/types/container#HostConfig
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

	autoRemove := false
	if hostConfig.AutoRemove {
		hostConfig.AutoRemove = false
		autoRemove = true
	}

	if a, ok := execCfg.Config["autoRemove"]; ok {
		if a, ok := a.(bool); ok {
			autoRemove = a
		}
	}

	exec := &DockerExecutor{
		step:            step,
		stdout:          os.Stdout,
		containerConfig: containerConfig,
		hostConfig:      hostConfig,
		autoRemove:      autoRemove,
	}

	if img, ok := execCfg.Config["image"]; ok {
		if img, ok := img.(string); ok {
			exec.image = img
			return exec, nil
		}
	}
	return nil, fmt.Errorf("image must be string")
}

func init() {
	Register("docker", CreateDockerExecutor)
}
