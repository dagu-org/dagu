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

var _ Executor = (*childWorkflow)(nil)
var _ ChildWorkflow = (*childWorkflow)(nil)

type childWorkflow struct {
	dag             *digraph.DAG
	lock            sync.Mutex
	childWorkflowID string
	params          string
	workDir         string
	stdout          io.Writer
	stderr          io.Writer
	cmd             *exec.Cmd
}

var ErrWorkingDirNotExist = fmt.Errorf("working directory does not exist")

func newChildWorkflow(
	ctx context.Context, step digraph.Step,
) (Executor, error) {
	config, err := EvalObject(ctx, struct {
		Name   string
		Params string
	}{
		Name:   step.ChildDAG.Name,
		Params: step.ChildDAG.Params,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to substitute string fields: %w", err)
	}

	env := GetEnv(ctx)
	sub, err := env.DB.GetDAG(ctx, config.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find child DAG %q: %w", config.Name, err)
	}

	dir, err := EvalString(ctx, step.Dir)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate working directory: %w", err)
	}

	if step.Dir != "" && !fileutil.FileExists(step.Dir) {
		return nil, ErrWorkingDirNotExist
	}

	return &childWorkflow{
		dag:     sub,
		params:  config.Params,
		workDir: dir,
	}, nil
}

func (e *childWorkflow) Run(ctx context.Context) error {
	executable, err := executablePath()
	if err != nil {
		return fmt.Errorf("failed to find executable path: %w", err)
	}

	if e.childWorkflowID == "" {
		return fmt.Errorf("execution ID is not set for child DAG")
	}

	e.lock.Lock()

	env := GetEnv(ctx)

	if env.Root.IsZero() {
		return fmt.Errorf("root execution ID is not set")
	}

	args := []string{
		"start",
		fmt.Sprintf("--root=%s", env.Root.String()),
		fmt.Sprintf("--parent=%s", env.ExecRef().String()),
		fmt.Sprintf("--workflow-id=%s", e.childWorkflowID),
		"--quiet",
		e.dag.Location,
	}

	if e.params != "" {
		args = append(args, "--")
		args = append(args, e.params)
	}

	cmd := exec.CommandContext(ctx, executable, args...) // nolint:gosec
	cmd.Dir = e.workDir
	cmd.Env = append(cmd.Env, env.AllEnvs()...)
	if e.stdout != nil {
		cmd.Stdout = e.stdout
	}
	if e.stderr != nil {
		// TODO: Separate stderr and stdout for child DAG to avoid mixing logger output
		cmd.Stderr = e.stderr
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	e.cmd = cmd

	logger.Info(ctx, "Executing child DAG",
		"workflowId", e.childWorkflowID,
		"target", e.dag.Name,
		"args", args,
	)

	err = cmd.Start()
	e.lock.Unlock()

	if err != nil {
		return fmt.Errorf("failed to start child DAG: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("child DAG exited with error: %w", err)
	}

	// get results from the child DAG
	result, err := env.DB.GetChildExecStatus(ctx, e.childWorkflowID, env.Root)
	if err != nil {
		return fmt.Errorf("failed to collect result for the child execucion (exec ID=%s root=%s): %w", e.childWorkflowID, env.Root, err)
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

func (e *childWorkflow) SetWorkflowID(id string) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.childWorkflowID = id
}

func (e *childWorkflow) SetStdout(out io.Writer) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.stdout = out
}

func (e *childWorkflow) SetStderr(out io.Writer) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.stderr = out
}

func (e *childWorkflow) Kill(sig os.Signal) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.cmd == nil || e.cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-e.cmd.Process.Pid, sig.(syscall.Signal))
}

func init() {
	Register(digraph.ExecutorTypeSubLegacy, newChildWorkflow)
	Register(digraph.ExecutorTypeSub, newChildWorkflow)
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
