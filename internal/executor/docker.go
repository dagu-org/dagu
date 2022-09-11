package executor

import (
	"context"
	"io"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/mitchellh/mapstructure"
	"github.com/yohamta/dagu/internal/dag"
)

type DockerExecutor struct {
	config *container.Config
	stdout io.Writer
}

func (e *DockerExecutor) SetStdout(out io.Writer) {
	e.stdout = out
}

func (e *DockerExecutor) SetStderr(out io.Writer) {
	e.stdout = out
}

func (e *DockerExecutor) Kill(sig os.Signal) error {
	// TODO: stop container
	return nil
}

func (e *DockerExecutor) Run() error {
	// TODO: start container
	return nil
}

func CreateDockerExecutor(ctx context.Context, step *dag.Step) (Executor, error) {
	step.Executor = "docker"
	cfg := &container.Config{}
	md, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result: cfg,
	})

	if err != nil {
		return nil, err
	}

	if err := md.Decode(step.ExecutorConfig); err != nil {
		return nil, err
	}

	// TODO: validate config if necessary

	return &DockerExecutor{
		stdout: os.Stdout,
		config: cfg,
	}, nil
}

func init() {
	Register("docker", CreateDockerExecutor)
}
