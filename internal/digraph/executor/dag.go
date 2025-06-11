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

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
)

var _ DAGExecutor = (*dagExecutor)(nil)

type dagExecutor struct {
	dag       *digraph.DAG
	lock      sync.Mutex
	workDir   string
	stdout    io.Writer
	stderr    io.Writer
	cmd       *exec.Cmd
	runParams RunParams
}

// Errors for DAG executor
var (
	ErrWorkingDirNotExist = fmt.Errorf("working directory does not exist")
)

func newDAGExecutor(
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

	return &dagExecutor{
		dag:     dag,
		workDir: step.Dir,
	}, nil
}

func (e *dagExecutor) Run(ctx context.Context) error {
	executable, err := executablePath()
	if err != nil {
		return fmt.Errorf("failed to find executable path: %w", err)
	}

	if e.runParams.RunID == "" {
		return fmt.Errorf("dag-run ID is not set")
	}

	e.lock.Lock()

	env := GetEnv(ctx)

	if env.RootDAGRun.Zero() {
		return fmt.Errorf("root dag-run ID is not set")
	}

	args := []string{
		"start",
		fmt.Sprintf("--root=%s", env.RootDAGRun.String()),
		fmt.Sprintf("--parent=%s", env.DAGRunRef().String()),
		fmt.Sprintf("--run-id=%s", e.runParams.RunID),
		"--no-queue",
		e.dag.Location,
	}

	if configFile := config.UsedConfigFile.Load(); configFile != nil {
		if configFile, ok := configFile.(string); ok {
			args = append(args, "--config")
			args = append(args, configFile)
		}
	}

	if e.runParams.Params != "" {
		args = append(args, "--")
		args = append(args, e.runParams.Params)
	}

	cmd := exec.CommandContext(ctx, executable, args...) // nolint:gosec
	cmd.Dir = e.workDir
	cmd.Env = append(cmd.Env, env.AllEnvs()...)
	if e.stdout != nil {
		cmd.Stdout = e.stdout
	}
	if e.stderr != nil {
		cmd.Stderr = e.stderr
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	e.cmd = cmd

	logger.Info(ctx, "Executing child DAG",
		"dagRunId", e.runParams.RunID,
		"target", e.dag.Name,
		"args", args,
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

func executablePath() (string, error) {
	if os.Getenv("DAGU_EXECUTABLE") != "" {
		return os.Getenv("DAGU_EXECUTABLE"), nil
	}
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}
	return executable, nil
}
