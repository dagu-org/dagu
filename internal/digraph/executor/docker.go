package executor

import (
	"context"
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
	container container.Container
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
	cfg, err := container.ParseMapConfig(ctx, execCfg.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse container config: %w", err)
	}

	return &docker{
		container: *cfg,
		step:      step,
		stdout:    os.Stdout,
		stderr:    os.Stderr,
	}, nil
}

func init() {
	Register("docker", newDocker)
}
