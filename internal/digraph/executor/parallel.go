package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
)

var _ ParallelExecutor = (*parallelExecutor)(nil)

type parallelExecutor struct {
	dag           *digraph.DAG
	lock          sync.Mutex
	workDir       string
	stdout        io.Writer
	stderr        io.Writer
	runParamsList []RunParams
}

func newParallelExecutor(
	ctx context.Context, step digraph.Step,
) (Executor, error) {
	cfg := struct {
		Name   string
		Params string
	}{
		Name:   step.ChildDAG.Name,
		Params: step.ChildDAG.Params,
	}

	env := GetEnv(ctx)
	dag, err := env.DB.GetDAG(ctx, cfg.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find DAG %q: %w", cfg.Name, err)
	}

	if step.Dir != "" && !fileutil.FileExists(step.Dir) {
		return nil, ErrWorkingDirNotExist
	}

	return &parallelExecutor{
		dag:     dag,
		workDir: step.Dir,
	}, nil
}

func (e *parallelExecutor) Run(ctx context.Context) error {
	executable, err := executablePath()
	if err != nil {
		return fmt.Errorf("failed to find executable path: %w", err)
	}

	return nil
}

func (e *parallelExecutor) SetParamsList(paramsList []RunParams) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.runParamsList = paramsList
}

func (e *parallelExecutor) SetStdout(out io.Writer) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.stdout = out
}

func (e *parallelExecutor) SetStderr(out io.Writer) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.stderr = out
}

func (e *parallelExecutor) Kill(sig os.Signal) error {
	e.lock.Lock()
	defer e.lock.Unlock()
}

func init() {
	Register(digraph.ExecutorTypeParallel, newParallelExecutor)
}
