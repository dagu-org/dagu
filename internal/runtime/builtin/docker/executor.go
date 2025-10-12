package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/signal"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime/executor"
)

var (
	ErrExecutorConfigRequired = errors.New("executor config is required")
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

var _ executor.Executor = (*docker)(nil)
var _ executor.ExitCoder = (*docker)(nil)

type containerClientCtxKey = struct{}
type registryAuthCtxKey = struct{}

// WithContainerClient creates a new context with a client for container
func WithContainerClient(ctx context.Context, cli *Client) context.Context {
	return context.WithValue(ctx, containerClientCtxKey{}, cli)
}

// getContainerClient retrieves the container client from the context.
func getContainerClient(ctx context.Context) *Client {
	if cli, ok := ctx.Value(containerClientCtxKey{}).(*Client); ok {
		return cli
	}
	return nil
}

// WithRegistryAuth creates a new context with registry authentication.
func WithRegistryAuth(ctx context.Context, auths map[string]*core.AuthConfig) context.Context {
	return context.WithValue(ctx, registryAuthCtxKey{}, auths)
}

// getRegistryAuth retrieves the registry authentication from the context.
func getRegistryAuth(ctx context.Context) map[string]*core.AuthConfig {
	if auths, ok := ctx.Value(registryAuthCtxKey{}).(map[string]*core.AuthConfig); ok {
		return auths
	}
	return nil
}

type docker struct {
	step      core.Step
	stdout    io.Writer
	stderr    io.Writer
	context   context.Context
	cancel    func()
	cfg       *Config
	container *Client
	mu        sync.Mutex
	exitCode  int
}

func (e *docker) SetStdout(out io.Writer) {
	e.stdout = out
}

func (e *docker) SetStderr(out io.Writer) {
	e.stderr = out
}

func (e *docker) Kill(sig os.Signal) error {
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
	if e.container == nil {
		return nil
	}

	if sig == syscall.SIGKILL {
		return e.container.Stop(sig)
	}
	if sig == syscall.SIGTERM && e.step.SignalOnStop != "" {
		sig = syscall.Signal(signal.GetSignalNum(e.step.SignalOnStop))
	}

	// Wait for max clean up time before forcefully killing the container
	go func() {
		env := core.GetEnv(e.context)
		<-time.After(env.DAG.MaxCleanUpTime)
		logger.Warn(e.context, "forcefully stopping container after max clean up time", "container", e.step.Name)
		_ = e.container.Stop(syscall.SIGKILL)
	}()

	return e.container.Stop(sig)
}

func (e *docker) Run(ctx context.Context) error {
	ctx, cancelFunc := context.WithCancel(ctx)
	e.context = ctx
	e.cancel = cancelFunc

	defer cancelFunc()

	// Wrap stderr with a tail writer to capture recent output for inclusion in
	// error messages.
	tw := executor.NewTailWriter(e.stderr, 0)
	e.stderr = tw

	cli := getContainerClient(ctx)
	if cli != nil {
		// If it exists, use the client from the context
		// This allows sharing the same container client across multiple executors.
		// Don't set WorkingDir - use the container's default working directory
		execOpts := ExecOptions{}

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

	if e.cfg == nil {
		return ErrExecutorConfigRequired
	}

	cli, err := InitializeClient(ctx, e.cfg)
	if err != nil {
		if tail := tw.Tail(); tail != "" {
			return fmt.Errorf("failed to setup container: %w\nrecent stderr (tail):\n%s", err, tail)
		}
		return fmt.Errorf("failed to setup container: %w", err)
	}

	e.container = cli
	defer e.container.Close(ctx)

	// Build command only when explicitly provided; otherwise use image default CMD/ENTRYPOINT.
	var cmd []string
	if e.step.Command != "" {
		cmd = append([]string{e.step.Command}, e.step.Args...)
	}

	exitCode, err := e.container.Run(ctx, cmd, e.stdout, e.stderr)

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

func newDocker(ctx context.Context, step core.Step) (executor.Executor, error) {
	execCfg := step.ExecutorConfig

	var cfg *Config
	if len(execCfg.Config) > 0 {
		// Get registry auth from context if available
		registryAuths := getRegistryAuth(ctx)
		c, err := LoadConfigFromMap(execCfg.Config, registryAuths)
		if err != nil {
			return nil, fmt.Errorf("failed to load container config: %w", err)
		}
		// Set ShouldStart to true for Step-level containers
		// This ensures the container is automatically created and started
		// if it does not exist or is stopped.
		c.ShouldStart = true
		cfg = c
	}

	return &docker{
		cfg:    cfg,
		step:   step,
		stdout: os.Stdout,
		stderr: os.Stderr,
	}, nil
}

func init() {
	executor.RegisterExecutor("docker", newDocker, nil)
	executor.RegisterExecutor("container", newDocker, nil)
}
