// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package executor

// See https://docs.docker.com/engine/api/sdk/

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/daguflow/dagu/internal/dag"
	"github.com/daguflow/dagu/internal/util"
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
	ctx, cancelFunc := context.WithCancel(context.Background())
	e.context = ctx
	e.cancel = cancelFunc

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

	removing := false
	removeContainer := func() {
		if !e.autoRemove || removing {
			return
		}
		removing = true
		err := cli.ContainerRemove(
			ctx, resp.ID, types.ContainerRemoveOptions{
				Force: true,
			},
		)
		util.LogErr("docker executor: remove container", err)
	}

	defer removeContainer()
	e.cancel = func() {
		removeContainer()
		cancelFunc()
	}

	if err := cli.ContainerStart(
		ctx, resp.ID, types.ContainerStartOptions{},
	); err != nil {
		return err
	}

	out, err := cli.ContainerLogs(
		ctx, resp.ID, types.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
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
