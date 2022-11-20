package executor

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/mitchellh/mapstructure"
	"github.com/yohamta/dagu/internal/dag"
)

type DockerExecutor struct {
	image   string
	step    *dag.Step
	config  *container.Config
	stdout  io.Writer
	context context.Context
	cancel  func()
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
	io.Copy(e.stdout, reader)

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: e.image,
		Cmd:   append([]string{e.step.Command}, e.step.Args...),
	}, nil, nil, nil, "")

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

	stdcopy.StdCopy(e.stdout, e.stdout, out)

	return nil
}

func CreateDockerExecutor(ctx context.Context, step *dag.Step) (Executor, error) {
	cfg := &container.Config{}
	execCfg := step.ExecutorConfig

	if cfg, ok := execCfg.Config["container"]; ok {
		md, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			Result: cfg,
		})

		if err != nil {
			return nil, err
		}

		if err := md.Decode(cfg); err != nil {
			return nil, err
		}
	}

	exec := &DockerExecutor{
		step:   step,
		stdout: os.Stdout,
		config: cfg,
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
