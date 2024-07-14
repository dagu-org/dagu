package executor

// See https://docs.docker.com/engine/api/sdk/

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/util"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

type docker struct {
	image           string
	pull            bool
	autoRemove      bool
	step            dag.Step
	containerConfig *container.Config
	hostConfig      *container.HostConfig
	stdout          io.Writer
	context         context.Context
	cancel          func()
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

func (e *docker) Run() error {
	ctx, fn := context.WithCancel(context.Background())
	e.context = ctx
	e.cancel = fn

	cli, err := client.NewClientWithOpts(
		client.FromEnv, client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return err
	}
	defer cli.Close()

	if e.pull {
		reader, err := cli.ImagePull(ctx, e.image, types.ImagePullOptions{})
		if err != nil {
			return err
		}
		_, err = io.Copy(e.stdout, reader)
		if err != nil {
			return err
		}
	}

	if e.image != "" {
		e.containerConfig.Image = e.image
	}
	e.containerConfig.Cmd = append([]string{e.step.Command}, e.step.Args...)

	resp, err := cli.ContainerCreate(
		ctx, e.containerConfig, e.hostConfig, nil, nil, "",
	)

	if err != nil {
		return err
	}

	if err := cli.ContainerStart(
		ctx, resp.ID, types.ContainerStartOptions{},
	); err != nil {
		return err
	}

	defer func() {
		if e.autoRemove {
			err := cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{})
			util.LogErr("docker executor: remove container", err)
		}
	}()

	out, err := cli.ContainerLogs(
		ctx, resp.ID, types.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow: true,
		},
	)
	if err != nil {
		return err
	}

	go func() {
		_, err = stdcopy.StdCopy(e.stdout, e.stdout, out)
		util.LogErr("docker executor: stdcopy", err)
	}()

	statusCh, errCh := cli.ContainerWait(
		ctx, resp.ID, container.WaitConditionNotRunning,
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

var errImageMustBeString = errors.New("image must be string")

func newDocker(
	_ context.Context, step dag.Step,
) (Executor, error) {
	containerConfig := &container.Config{}
	hostConfig := &container.HostConfig{}
	execCfg := step.ExecutorConfig

	if cfg, ok := execCfg.Config["container"]; ok {
		// nolint
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
		// nolint
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

	pull := true
	if p, ok := execCfg.Config["pull"]; ok {
		if p, ok := p.(bool); ok {
			pull = p
		}
	}

	exec := &docker{
		pull:            pull,
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
	return nil, errImageMustBeString
}

func init() {
	Register("docker", newDocker)
}
