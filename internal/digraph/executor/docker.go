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

type containerClientCtxKey = struct{}
type registryAuthCtxKey = struct{}

// WithContainerClient creates a new context with a client for container
func WithContainerClient(ctx context.Context, cli *container.Client) context.Context {
	return context.WithValue(ctx, containerClientCtxKey{}, cli)
}

// getContainerClient retrieves the container client from the context.
func getContainerClient(ctx context.Context) *container.Client {
	if cli, ok := ctx.Value(containerClientCtxKey{}).(*container.Client); ok {
		return cli
	}
	return nil
}

// WithRegistryAuth creates a new context with registry authentication.
func WithRegistryAuth(ctx context.Context, auths map[string]*digraph.AuthConfig) context.Context {
	return context.WithValue(ctx, registryAuthCtxKey{}, auths)
}

// getRegistryAuth retrieves the registry authentication from the context.
func getRegistryAuth(ctx context.Context) map[string]*digraph.AuthConfig {
	if auths, ok := ctx.Value(registryAuthCtxKey{}).(map[string]*digraph.AuthConfig); ok {
		return auths
	}
	return nil
}

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

	// Wrap stderr with a tail writer to capture recent output for inclusion in
	// error messages.
	tw := newTailWriter(e.stderr, 0)
	e.stderr = tw

	cli := getContainerClient(ctx)
	if cli != nil {
		// If it exists, use the client from the context
		// This allows sharing the same container client across multiple executors.
		// Don't set WorkingDir - use the container's default working directory
		execOpts := container.ExecOptions{}

		// Build command only when a command is explicitly provided.
		// If command is empty, avoid passing an empty string which overrides image CMD.
		var cmd []string
		if e.step.Command != "" {
			cmd = append([]string{e.step.Command}, e.step.Args...)
		}

		exitCode, err := cli.Exec(
			ctx,
			cmd,
			e.stdout, e.stderr,
			execOpts,
		)
		e.mu.Lock()
		e.exitCode = exitCode
		e.mu.Unlock()
		if err != nil {
			if tail := tw.Tail(); tail != "" {
				return fmt.Errorf("%w\nrecent stderr (tail):\n%s", err, tail)
			}
		}
		return err
	}

	if err := e.container.Init(ctx); err != nil {
		if tail := tw.Tail(); tail != "" {
			return fmt.Errorf("failed to setup container: %w\nrecent stderr (tail):\n%s", err, tail)
		}
		return fmt.Errorf("failed to setup container: %w", err)
	}
	defer e.container.Close(ctx)

	// Build command only when explicitly provided; otherwise use image default CMD/ENTRYPOINT.
	var cmd []string
	if e.step.Command != "" {
		cmd = append([]string{e.step.Command}, e.step.Args...)
	}

	exitCode, err := e.container.Run(
		ctx,
		cmd,
		e.stdout, e.stderr,
	)

	e.mu.Lock()
	e.exitCode = exitCode
	e.mu.Unlock()

	if err != nil {
		if tail := tw.Tail(); tail != "" {
			return fmt.Errorf("%w\nrecent stderr (tail):\n%s", err, tail)
		}
	}
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
		// Get registry auth from context if available
		registryAuths := getRegistryAuth(ctx)
		ct, err = container.NewFromMapConfigWithAuth(execCfg.Config, registryAuths)
		if err != nil {
			return nil, fmt.Errorf("failed to parse executor config: %w", err)
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
