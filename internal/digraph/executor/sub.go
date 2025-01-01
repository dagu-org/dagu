// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/google/uuid"
)

type subWorkflow struct {
	subDAG    string
	cmd       *exec.Cmd
	lock      sync.Mutex
	requestID string
	writer    io.Writer
}

var errWorkingDirNotExist = fmt.Errorf("working directory does not exist")

func newSubWorkflow(
	ctx context.Context, step digraph.Step,
) (Executor, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	dagCtx := digraph.GetContext(ctx)

	config, err := cmdutil.SubstituteStringFields(struct {
		Name   string
		Params string
	}{
		Name:   step.SubWorkflow.Name,
		Params: step.SubWorkflow.Params,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to substitute string fields: %w", err)
	}

	subDAG, err := dagCtx.GetDAGByName(config.Name)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to find subworkflow %q: %w", config.Name, err,
		)
	}

	requestID, err := generateRequestID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate request ID: %w", err)
	}

	args := []string{
		"start",
		fmt.Sprintf("--requestID=%s", requestID),
		"--quiet",
		fmt.Sprintf("--params=%q", config.Params),
		subDAG.Location,
	}

	cmd := exec.CommandContext(ctx, executable, args...)
	if len(step.Dir) > 0 && !fileutil.FileExists(step.Dir) {
		return nil, errWorkingDirNotExist
	}
	cmd.Dir = step.Dir
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Env = append(cmd.Env, dagCtx.DAG.Env...)

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
		cmd:       cmd,
		requestID: requestID,
		subDAG:    subDAG.Location,
	}, nil
}

func (e *subWorkflow) Run(ctx context.Context) error {
	e.lock.Lock()
	err := e.cmd.Start()
	e.lock.Unlock()
	if err != nil {
		return err
	}
	if err := e.cmd.Wait(); err != nil {
		return err
	}

	// get results from the subworkflow
	dagCtx := digraph.GetContext(ctx)
	result, err := dagCtx.GetResult(e.subDAG, e.requestID)
	if err != nil {
		return fmt.Errorf("failed to collect result: %w", err)
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal outputs: %w", err)
	}

	if _, err := e.writer.Write(jsonData); err != nil {
		return fmt.Errorf("failed to write outputs: %w", err)
	}

	return nil
}

func (e *subWorkflow) SetStdout(out io.Writer) {
	e.cmd.Stdout = out
	e.writer = out
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

// generateRequestID generates a new request ID.
// For simplicity, we use UUIDs as request IDs.
func generateRequestID() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}
