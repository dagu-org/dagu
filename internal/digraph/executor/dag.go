package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
)

var _ DAGExecutor = (*dagExecutor)(nil)
var _ NodeStatusDeterminer = (*dagExecutor)(nil)

type dagExecutor struct {
	child     *ChildDAGExecutor
	lock      sync.Mutex
	workDir   string
	stdout    io.Writer
	stderr    io.Writer
	cmd       *exec.Cmd
	runParams RunParams
	result    *digraph.RunStatus
}

// Errors for DAG executor
var (
	ErrWorkingDirNotExist = fmt.Errorf("working directory does not exist")
)

func newDAGExecutor(
	ctx context.Context, step digraph.Step,
) (Executor, error) {
	if step.ChildDAG == nil {
		return nil, fmt.Errorf("child DAG configuration is missing")
	}

	child, err := NewChildDAGExecutor(ctx, step.ChildDAG.Name)
	if err != nil {
		return nil, err
	}

	dir := GetEnv(ctx).WorkingDir
	if dir != "" && !fileutil.FileExists(dir) {
		return nil, ErrWorkingDirNotExist
	}

	return &dagExecutor{
		child:   child,
		workDir: dir,
	}, nil
}

func (e *dagExecutor) Run(ctx context.Context) error {
	// Ensure cleanup happens even if there's an error
	defer func() {
		if err := e.child.Cleanup(ctx); err != nil {
			logger.Error(ctx, "Failed to cleanup child DAG executor", "error", err)
		}
	}()

	e.lock.Lock()

	cmd, err := e.child.BuildCommand(ctx, e.runParams, e.workDir)
	if err != nil {
		e.lock.Unlock()
		return err
	}

	if e.stdout != nil {
		cmd.Stdout = e.stdout
	}
	if e.stderr != nil {
		cmd.Stderr = e.stderr
	}

	e.cmd = cmd

	logger.Info(ctx, "Executing child DAG",
		"dagRunId", e.runParams.RunID,
		"target", e.child.DAG.Name,
	)

	err = cmd.Start()
	e.lock.Unlock()

	if err != nil {
		return fmt.Errorf("failed to start child dag-run: %w", err)
	}

	// Wait for the command to complete
	waitErr := cmd.Wait()

	// Always get the actual status from the child dag-run, regardless of exit code
	env := GetEnv(ctx)
	result, err := env.DB.GetChildDAGRunStatus(ctx, e.runParams.RunID, env.RootDAGRun)
	if err != nil {
		return fmt.Errorf("failed to find result for the child dag-run %q: %w", e.runParams.RunID, err)
	}

	e.result = result

	if result.Status.IsSuccess() {
		if waitErr != nil {
			logger.Warn(ctx, "Child DAG completed with exit code but no error",
				"dagRunId", e.runParams.RunID,
				"err", waitErr,
			)
		} else {
			logger.Info(ctx, "Child DAG completed successfully", "dagRunId", e.runParams.RunID)
		}
	} else {
		return fmt.Errorf("child dag-run failed with status: %s", result.Status)
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal outputs: %w", err)
	}

	// add a newline at the end of the JSON output
	jsonData = append(jsonData, '\n')

	if e.stdout != nil {
		if _, err := e.stdout.Write(jsonData); err != nil {
			return fmt.Errorf("failed to write outputs: %w", err)
		}
	}

	return nil
}

// DetermineNodeStatus implements NodeStatusDeterminer.
func (e *dagExecutor) DetermineNodeStatus(_ context.Context) (status.NodeStatus, error) {
	if e.result == nil {
		return status.NodeError, fmt.Errorf("no result available for node status determination")
	}

	// Check if the status is partial success or success
	// For error cases, we return an error with the status
	switch e.result.Status {
	case status.Success:
		return status.NodeSuccess, nil
	case status.PartialSuccess:
		return status.NodePartialSuccess, nil
	case status.None, status.Running, status.Error, status.Cancel, status.Queued:
		return status.NodeError, fmt.Errorf("child DAG run %s failed with status: %s", e.result.DAGRunID, e.result.Status)
	default:
		// This should never happen, but satisfies the exhaustive check
		return status.NodeError, fmt.Errorf("child DAG run %s failed with unknown status: %s", e.result.DAGRunID, e.result.Status)
	}
}

func (e *dagExecutor) SetParams(params RunParams) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.runParams = params
}

func (e *dagExecutor) SetStdout(out io.Writer) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.stdout = out
}

func (e *dagExecutor) SetStderr(out io.Writer) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.stderr = out
}

func (e *dagExecutor) Kill(sig os.Signal) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	return killProcessGroup(e.cmd, sig)
}

func init() {
	Register(digraph.ExecutorTypeDAGLegacy, newDAGExecutor)
	Register(digraph.ExecutorTypeDAG, newDAGExecutor)
}
