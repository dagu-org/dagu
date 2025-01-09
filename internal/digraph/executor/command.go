package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
)

var _ Executor = (*commandExecutor)(nil)
var _ ExitCoder = (*commandExecutor)(nil)

type commandExecutor struct {
	cmd      *exec.Cmd
	lock     sync.Mutex
	exitCode int
}

// ExitCode implements ExitCoder.
func (e *commandExecutor) ExitCode() int {
	return e.exitCode
}

func (e *commandExecutor) Run(_ context.Context) error {
	e.lock.Lock()
	err := e.cmd.Start()
	e.lock.Unlock()
	if err != nil {
		e.exitCode = exitCodeFromError(err)
		return err
	}
	if err := e.cmd.Wait(); err != nil {
		e.exitCode = exitCodeFromError(err)
		return err
	}
	return nil
}

func (e *commandExecutor) SetStdout(out io.Writer) {
	e.cmd.Stdout = out
}

func (e *commandExecutor) SetStderr(out io.Writer) {
	e.cmd.Stderr = out
}

func (e *commandExecutor) Kill(sig os.Signal) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.cmd == nil || e.cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-e.cmd.Process.Pid, sig.(syscall.Signal))
}

func init() {
	Register("", newCommand)
	Register("command", newCommand)
}

func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	var exitCode int
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else {
		exitCode = 1
	}
	return exitCode
}

func newCommand(ctx context.Context, step digraph.Step) (Executor, error) {
	if len(step.Dir) > 0 && !fileutil.FileExists(step.Dir) {
		return nil, fmt.Errorf("directory %q does not exist", step.Dir)
	}

	stepContext := digraph.GetStepContext(ctx)

	cmd, err := createCommand(ctx, step)
	if err != nil {
		return nil, fmt.Errorf("failed to create command: %w", err)
	}
	cmd.Env = append(cmd.Env, stepContext.AllEnvs()...)
	cmd.Dir = step.Dir

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	return &commandExecutor{cmd: cmd}, nil
}

func createCommand(ctx context.Context, step digraph.Step) (*exec.Cmd, error) {
	shellCommand := cmdutil.GetShellCommand(step.Shell)
	shellCmdArgs := step.ShellCmdArgs
	if shellCommand == "" || shellCmdArgs == "" {
		return createDirectCommand(ctx, step, step.Args), nil
	}
	return createShellCommand(ctx, shellCommand, shellCmdArgs), nil
}

// createDirectCommand creates a command that runs directly without a shell
func createDirectCommand(ctx context.Context, step digraph.Step, args []string) *exec.Cmd {
	// nolint: gosec
	return exec.CommandContext(ctx, step.Command, args...)
}

// createShellCommand creates a command that runs through a shell
func createShellCommand(ctx context.Context, shell, shellCmd string) *exec.Cmd {
	return exec.CommandContext(ctx, shell, "-c", shellCmd)
}
