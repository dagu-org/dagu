// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
)

type subWorkflow struct {
	cmd  *exec.Cmd
	lock sync.Mutex
}

var errWorkingDirNotExist = fmt.Errorf("working directory does not exist")

func newSubWorkflow(
	ctx context.Context, step digraph.Step,
) (Executor, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	dagCtx, err := digraph.GetContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get dag context: %w", err)
	}

	sugDAG, err := dagCtx.Finder.Find(ctx, step.SubWorkflow.Name)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to find subworkflow %q: %w", step.SubWorkflow.Name, err,
		)
	}

	params := os.ExpandEnv(step.SubWorkflow.Params)

	args := []string{
		"start",
		fmt.Sprintf("--params=%q", params),
		sugDAG.Location,
	}

	cmd := exec.CommandContext(ctx, executable, args...)
	if len(step.Dir) > 0 && !fileutil.FileExists(step.Dir) {
		return nil, errWorkingDirNotExist
	}
	cmd.Dir = step.Dir
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Env = append(cmd.Env, step.Variables...)

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

	return &subWorkflow{
		cmd: cmd,
	}, nil
}

func (e *subWorkflow) Run() error {
	e.lock.Lock()
	err := e.cmd.Start()
	e.lock.Unlock()
	if err != nil {
		return err
	}
	return e.cmd.Wait()
}

func (e *subWorkflow) SetStdout(out io.Writer) {
	e.cmd.Stdout = out
}

func (e *subWorkflow) SetStderr(out io.Writer) {
	e.cmd.Stderr = out
}

func (e *subWorkflow) Kill(sig os.Signal) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.cmd == nil || e.cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-e.cmd.Process.Pid, sig.(syscall.Signal))
}

func init() {
	Register(digraph.ExecutorTypeSubWorkflow, newSubWorkflow)
}
