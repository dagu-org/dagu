// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
)

type commandExecutor struct {
	cmd  *exec.Cmd
	lock sync.Mutex
}

func newCommand(ctx context.Context, step digraph.Step) (Executor, error) {
	if len(step.Dir) > 0 && !fileutil.FileExists(step.Dir) {
		return nil, fmt.Errorf("directory %q does not exist", step.Dir)
	}

	dagContext, err := digraph.GetContext(ctx)
	if err != nil {
		return nil, err
	}

	cmd := createCommand(ctx, step)
	cmd.Dir = step.Dir
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Env = append(cmd.Env, dagContext.DAG.Env...)
	cmd.Env = append(cmd.Env, step.Variables...)
	cmd.Env = append(cmd.Env, dagContext.Envs.All()...)

	// Get output variables from the step context and set them as environment
	stepCtx := digraph.GetStepContext(ctx)
	if stepCtx != nil && stepCtx.OutputVariables != nil {
		stepCtx.OutputVariables.Range(func(_, value any) bool {
			cmd.Env = append(cmd.Env, value.(string))
			return true
		})
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	return &commandExecutor{
		cmd: cmd,
	}, nil
}

func createCommand(ctx context.Context, step digraph.Step) *exec.Cmd {
	shellCommand := cmdutil.GetShellCommand(step.Shell)

	if shellCommand == "" {
		return createDirectCommand(ctx, step)
	}
	return createShellCommand(ctx, shellCommand, step)
}

// createDirectCommand creates a command that runs directly without a shell
func createDirectCommand(ctx context.Context, step digraph.Step) *exec.Cmd {
	// nolint: gosec
	return exec.CommandContext(ctx, step.Command, step.Args...)
}

// createShellCommand creates a command that runs through a shell
func createShellCommand(ctx context.Context, shell string, step digraph.Step) *exec.Cmd {
	command := buildCommandString(step.Command, step.Args)
	return exec.CommandContext(ctx, shell, "-c", command)
}

// buildCommandString combines the command and arguments into a single string
func buildCommandString(command string, args []string) string {
	return fmt.Sprintf("%s %s", command, strings.Join(args, " "))
}

func (e *commandExecutor) Run(_ context.Context) error {
	e.lock.Lock()
	err := e.cmd.Start()
	e.lock.Unlock()
	if err != nil {
		return err
	}
	return e.cmd.Wait()
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
