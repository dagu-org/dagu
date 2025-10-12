package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
)

var _ scheduler.DAGExecutor = (*dagExecutor)(nil)
var _ scheduler.NodeStatusDeterminer = (*dagExecutor)(nil)

type dagExecutor struct {
	child     *ChildDAGExecutor
	lock      sync.Mutex
	workDir   string
	stdout    io.Writer
	stderr    io.Writer
	runParams scheduler.RunParams
	step      digraph.Step
	result    *digraph.RunStatus
}

// Errors for DAG executor
var (
	ErrWorkingDirNotExist = fmt.Errorf("working directory does not exist")
)

func newDAGExecutor(ctx context.Context, step digraph.Step) (digraph.Executor, error) {
	if step.ChildDAG == nil {
		return nil, fmt.Errorf("child DAG configuration is missing")
	}

	child, err := NewChildDAGExecutor(ctx, step.ChildDAG.Name)
	if err != nil {
		return nil, err
	}

	dir := digraph.GetEnv(ctx).WorkingDir
	if dir != "" && !fileutil.FileExists(dir) {
		return nil, ErrWorkingDirNotExist
	}

	return &dagExecutor{
		child:   child,
		workDir: dir,
		step:    step,
	}, nil
}

func (e *dagExecutor) Run(ctx context.Context) error {
	// Ensure cleanup happens even if there's an error
	defer func() {
		if err := e.child.Cleanup(ctx); err != nil {
			logger.Error(ctx, "Failed to cleanup child DAG executor", "err", err)
		}
	}()

	result, execErr := e.child.ExecuteWithResult(ctx, e.runParams, e.workDir)
	if result != nil {
		e.lock.Lock()
		e.result = result
		e.lock.Unlock()
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal outputs: %w", execErr)
	}

	// Add a newline at the end of the JSON output
	jsonData = append(jsonData, '\n')

	if e.stdout != nil {
		if _, err := e.stdout.Write(jsonData); err != nil {
			return fmt.Errorf("failed to write outputs: %w", err)
		}
	}

	return execErr
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

func (e *dagExecutor) SetParams(params scheduler.RunParams) {
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
	// Kill all child processes (both local and distributed)
	if e.child != nil {
		return e.child.Kill(sig)
	}

	return nil
}

func init() {
	digraph.RegisterExecutor(digraph.ExecutorTypeDAGLegacy, newDAGExecutor, nil)
	digraph.RegisterExecutor(digraph.ExecutorTypeDAG, newDAGExecutor, nil)
}
