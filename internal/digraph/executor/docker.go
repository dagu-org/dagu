package executor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

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
var _ ExitCoder = (*docker)(nil)

type docker struct {
	step      digraph.Step
	stdout    io.Writer
	stderr    io.Writer
	context   context.Context
	cancel    func()
	container *container.Client
	mu        sync.Mutex
	exitCode  int
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

	if err := e.container.Init(ctx); err != nil {
		return fmt.Errorf("failed to setup container: %w", err)
	}
	defer e.container.Close(ctx)

	exitCode, err := e.container.Run(
		ctx,
		append([]string{e.step.Command}, e.step.Args...),
		e.stdout, e.stderr,
	)

	e.mu.Lock()
	e.exitCode = exitCode
	e.mu.Unlock()

	return err
}

// ExitCode implements ExitCoder.
func (e *docker) ExitCode() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.exitCode
}

func newDocker(
	ctx context.Context, step digraph.Step,
) (Executor, error) {
	execCfg := step.ExecutorConfig

	var ct *container.Client

	if len(execCfg.Config) > 0 {
		var err error
		ct, err = container.NewFromMapConfig(execCfg.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse executor config: %w", err)
		}
	} else {
		env := GetEnv(ctx)
		if env.DAG.Container == nil {
			return nil, ErrExecutorConfigRequired
		}
		var err error
		ct, err = container.NewFromContainerConfig(*env.DAG.Container)
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
