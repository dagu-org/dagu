package executor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/dagu-org/dagu/internal/container"
	"github.com/dagu-org/dagu/internal/digraph"
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
	step      digraph.Step
	stdout    io.Writer
	stderr    io.Writer
	context   context.Context
	cancel    func()
	container *container.Container
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

func (e *docker) Run(ctx context.Context) error {
	ctx, cancelFunc := context.WithCancel(ctx)
	e.context = ctx
	e.cancel = cancelFunc

	defer cancelFunc()

	if err := e.container.Open(ctx); err != nil {
		return fmt.Errorf("failed to setup container: %w", err)
	}
	defer e.container.Close()

	return e.container.Run(
		ctx,
		append([]string{e.step.Command}, e.step.Args...),
		e.stdout, e.stderr,
	)
}

func newDocker(
	ctx context.Context, step digraph.Step,
) (Executor, error) {
	execCfg := step.ExecutorConfig

	var ct *container.Container

	if len(execCfg.Config) > 0 {
		var err error
		ct, err = container.ParseMapConfig(execCfg.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse executor config: %w", err)
		}
	} else {
		env := GetEnv(ctx)
		if env.DAG.Container == nil {
			return nil, ErrExecutorConfigRequired
		}
		var err error
		ct, err = container.ParseContainer(*env.DAG.Container)
		if err != nil {
			return nil, fmt.Errorf("failed to parse DAG container config: %w", err)
		}
	}

	return &docker{
		container: ct,
		step:      step,
		stdout:    os.Stdout,
		stderr:    os.Stderr,
	}, nil
}

var (
	ErrExecutorConfigRequired = errors.New("executor config is required")
)

func init() {
	Register("docker", newDocker)
}
