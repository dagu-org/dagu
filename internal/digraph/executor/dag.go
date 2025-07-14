package executor

import (
	"context"
	"fmt"
	"io"
	"os"
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

	// Track distributed execution state BEFORE execution starts
	if e.child.ShouldUseDistributedExecution() {
		e.lock.Lock()
		e.isDistributed = true
		e.childDAGRunID = e.runParams.RunID
		e.lock.Unlock()
	}

	// Execute using the simplified API
	err := e.child.Execute(ctx, e.runParams, e.workDir, e.stdout)

	return err
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
	isDistributed := e.isDistributed
	childDAGRunID := e.childDAGRunID
	e.lock.Unlock()

	// If running distributed, request cancellation
	if isDistributed && childDAGRunID != "" {
		ctx := context.Background()
		if err := e.env.DB.RequestChildCancel(ctx, childDAGRunID, e.env.RootDAGRun); err != nil {
			logger.Error(ctx, "Failed to request child DAG cancellation",
				"dagRunId", childDAGRunID, "err", err)
			return err
		}
		return nil
	}

	// For local processes, call Kill on the child executor
	if e.child != nil {
		return e.child.Kill(sig)
	}

	return nil
}

func init() {
	Register(digraph.ExecutorTypeDAGLegacy, newDAGExecutor)
	Register(digraph.ExecutorTypeDAG, newDAGExecutor)
}
