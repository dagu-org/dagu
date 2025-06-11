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

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
)

var _ DAGExecutor = (*dagExecutor)(nil)

type dagExecutor struct {
	child     *ChildDAGExecutor
	lock      sync.Mutex
	workDir   string
	stdout    io.Writer
	stderr    io.Writer
	cmd       *exec.Cmd
	runParams RunParams
	step      digraph.Step
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

	if step.Dir != "" && !fileutil.FileExists(step.Dir) {
		return nil, ErrWorkingDirNotExist
	}

	return &dagExecutor{
		child:   child,
		workDir: step.Dir,
		step:    step,
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

	// Apply resource limits if available
	env := GetEnv(ctx)
	if env.ResourceController != nil && e.step.Resources != nil {
		// Generate a unique name for this child DAG execution
		stepName := fmt.Sprintf("%s-%s-%s", env.DAG.Name, e.step.Name, e.runParams.RunID)
		
		err = env.ResourceController.StartProcess(ctx, cmd, e.step.Resources, stepName)
	} else {
		err = cmd.Start()
	}
	e.lock.Unlock()

	if err != nil {
		return fmt.Errorf("failed to start child dag-run: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("child dag-run failed: %w", err)
	}

	// get results from the child dag-run
	result, err := env.DB.GetChildDAGRunStatus(ctx, e.runParams.RunID, env.RootDAGRun)
	if err != nil {
		return fmt.Errorf("failed to find result for the child dag-run %q: %w", e.runParams.RunID, err)
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal outputs: %w", err)
	}

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
	if e.cmd == nil || e.cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-e.cmd.Process.Pid, sig.(syscall.Signal))
}

func init() {
	Register(digraph.ExecutorTypeDAGLegacy, newDAGExecutor)
	Register(digraph.ExecutorTypeDAG, newDAGExecutor)
}
