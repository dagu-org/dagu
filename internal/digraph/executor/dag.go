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
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
)

var _ DAGExecutor = (*dagExecutor)(nil)

type dagExecutor struct {
	child         *ChildDAGExecutor
	lock          sync.Mutex
	workDir       string
	stdout        io.Writer
	stderr        io.Writer
	cmd           *exec.Cmd
	runParams     RunParams
	step          digraph.Step
	isDistributed bool
	childDAGRunID string
	env           Env
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
		step:    step,
		env:     GetEnv(ctx),
	}, nil
}

func (e *dagExecutor) Run(ctx context.Context) error {
	// Ensure cleanup happens even if there's an error
	defer func() {
		if err := e.child.Cleanup(ctx); err != nil {
			logger.Error(ctx, "Failed to cleanup child DAG executor", "err", err)
		}
	}()

	// Check if we should use distributed execution
	if e.child.ShouldUseDistributedExecution() {
		logger.Info(ctx, "Worker selector specified for child DAG execution",
			"dag", e.child.DAG.Name,
			"workerSelector", e.step.WorkerSelector,
		)

		// Try distributed execution
		err := e.runDistributed(ctx)
		if err != nil {
			// Distributed execution was requested but failed - return error
			return fmt.Errorf("distributed execution failed for DAG %q: %w", e.child.DAG.Name, err)
		}
		// Distributed execution succeeded
		return nil
	}

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

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("child dag-run failed: %w", err)
	}

	// get results from the child dag-run
	env := GetEnv(ctx)
	result, err := env.DB.GetChildDAGRunStatus(ctx, e.runParams.RunID, env.RootDAGRun)
	if err != nil {
		return fmt.Errorf("failed to find result for the child dag-run %q: %w", e.runParams.RunID, err)
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

	// If running distributed, request cancellation
	if e.isDistributed && e.childDAGRunID != "" {
		ctx := context.Background()
		if err := e.env.DB.RequestChildCancel(ctx, e.childDAGRunID, e.env.RootDAGRun); err != nil {
			logger.Error(ctx, "Failed to request child DAG cancellation",
				"dagRunId", e.childDAGRunID, "err", err)
		}
	}

	// Still kill local process if any
	return killProcessGroup(e.cmd, sig)
}

// runDistributed attempts to execute the child DAG via the coordinator
func (e *dagExecutor) runDistributed(ctx context.Context) error {
	// Mark as distributed and store the child DAG run ID
	e.lock.Lock()
	e.isDistributed = true
	e.childDAGRunID = e.runParams.RunID
	e.lock.Unlock()

	// Create distributed executor
	distExec := NewDistributedExecutor(ctx)

	// Dispatch to coordinator
	if err := distExec.DispatchToCoordinator(ctx, e.child, e.runParams); err != nil {
		return err
	}

	// Wait for distributed execution to complete
	return distExec.WaitForCompletion(ctx, e.runParams.RunID, e.stdout)
}

func init() {
	Register(digraph.ExecutorTypeDAGLegacy, newDAGExecutor)
	Register(digraph.ExecutorTypeDAG, newDAGExecutor)
}
