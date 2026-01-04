package dag

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/runtime/executor"
)

var _ executor.DAGExecutor = (*dagExecutor)(nil)
var _ executor.NodeStatusDeterminer = (*dagExecutor)(nil)

type dagExecutor struct {
	child     *executor.SubDAGExecutor
	lock      sync.Mutex
	workDir   string
	stdout    io.Writer
	stderr    io.Writer
	runParams executor.RunParams
	step      core.Step
	result    *execution.RunStatus
	cancel    context.CancelFunc
}

// Errors for DAG executor
var (
	ErrWorkingDirNotExist   = fmt.Errorf("working directory does not exist")
	ErrHITLStepsWithWorker = fmt.Errorf("sub-DAG with HITL steps cannot be dispatched to workers")
)

func newDAGExecutor(ctx context.Context, step core.Step) (executor.Executor, error) {
	if step.SubDAG == nil {
		return nil, fmt.Errorf("sub DAG configuration is missing")
	}

	child, err := executor.NewSubDAGExecutor(ctx, step.SubDAG.Name)
	if err != nil {
		return nil, err
	}

	// Validate: sub-DAGs with HITL steps cannot be dispatched to workers
	if len(step.WorkerSelector) > 0 && child.DAG.HasHITLSteps() {
		return nil, fmt.Errorf("%w: %s", ErrHITLStepsWithWorker, step.SubDAG.Name)
	}

	dir := runtime.GetEnv(ctx).WorkingDir
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
	ctx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	defer cancel()

	// Ensure cleanup happens even if there's an error
	defer func() {
		if err := e.child.Cleanup(ctx); err != nil {
			logger.Error(ctx, "Failed to cleanup sub DAG executor", tag.Error(err))
		}
	}()

	result, execErr := e.child.Execute(ctx, e.runParams, e.workDir)
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
func (e *dagExecutor) DetermineNodeStatus() (core.NodeStatus, error) {
	if e.result == nil {
		return core.NodeFailed, fmt.Errorf("no result available for node status determination")
	}

	// Check if the status is partial success or success
	// For error cases, we return an error with the status
	switch e.result.Status {
	case core.Succeeded:
		return core.NodeSucceeded, nil
	case core.PartiallySucceeded:
		return core.NodePartiallySucceeded, nil
	case core.Wait:
		// Sub-DAG is waiting for human approval (HITL)
		// Propagate the waiting status to the parent
		return core.NodeWaiting, nil
	case core.NotStarted, core.Running, core.Failed, core.Aborted, core.Queued:
		return core.NodeFailed, fmt.Errorf("sub DAG run %s failed with status: %s", e.result.DAGRunID, e.result.Status)
	default:
		// This should never happen, but satisfies the exhaustive check
		return core.NodeFailed, fmt.Errorf("sub DAG run %s failed with unknown status: %s", e.result.DAGRunID, e.result.Status)
	}
}

func (e *dagExecutor) SetParams(params executor.RunParams) {
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
	var err error
	if e.child != nil {
		err = e.child.Kill(sig)
	}
	if e.cancel != nil {
		e.cancel()
	}
	return err
}

func init() {
	caps := core.ExecutorCapabilities{
		SubDAG:         true,
		WorkerSelector: true,
	}
	executor.RegisterExecutor("subworkflow", newDAGExecutor, nil, caps)
	executor.RegisterExecutor("dag", newDAGExecutor, nil, caps)
}
