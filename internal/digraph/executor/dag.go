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

	// Execute using the simplified API
	err := e.child.Execute(ctx, e.runParams, e.workDir, e.stdout)

	// Track distributed execution state for Kill method
	if e.child.ShouldUseDistributedExecution() {
		e.lock.Lock()
		e.isDistributed = true
		e.childDAGRunID = e.runParams.RunID
		e.lock.Unlock()
	}

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
	defer e.lock.Unlock()

	// If running distributed, request cancellation
	if e.isDistributed && e.childDAGRunID != "" {
		ctx := context.Background()
		if err := e.env.DB.RequestChildCancel(ctx, e.childDAGRunID, e.env.RootDAGRun); err != nil {
			logger.Error(ctx, "Failed to request child DAG cancellation",
				"dagRunId", e.childDAGRunID, "err", err)
		}
	}

	// For local processes, the Kill logic is handled inside ChildDAGExecutor
	return nil
}

func init() {
	Register(digraph.ExecutorTypeDAGLegacy, newDAGExecutor)
	Register(digraph.ExecutorTypeDAG, newDAGExecutor)
}
