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

var _ Executor = (*subDAG)(nil)
var _ SubDAG = (*subDAG)(nil)

type subDAG struct {
	dag       *digraph.DAG
	lock      sync.Mutex
	requestID string
	params    string
	workDir   string
	stdout    io.Writer
	stderr    io.Writer
	cmd       *exec.Cmd
}

var ErrWorkingDirNotExist = fmt.Errorf("working directory does not exist")

func newSubDAG(
	ctx context.Context, step digraph.Step,
) (Executor, error) {
	config, err := digraph.EvalObject(ctx, struct {
		Name   string
		Params string
	}{
		Name:   step.SubDAG.Name,
		Params: step.SubDAG.Params,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to substitute string fields: %w", err)
	}

	sub, err := digraph.GetDAGByName(ctx, config.Name)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to find sub DAG %q: %w", config.Name, err,
		)
	}

	// TODO: Evaluate the working directory
	dir := step.Dir
	if step.Dir != "" && !fileutil.FileExists(step.Dir) {
		return nil, ErrWorkingDirNotExist
	}

	return &subDAG{
		dag:     sub,
		params:  config.Params,
		workDir: dir,
	}, nil
}

func (e *subDAG) Run(ctx context.Context) error {
	executable, err := executablePath()
	if err != nil {
		return fmt.Errorf("failed to find executable path: %w", err)
	}

	if e.requestID == "" {
		return fmt.Errorf("request ID is not set for sub-DAG")
	}

	e.lock.Lock()

	c := digraph.GetExecContext(ctx)

	args := []string{
		"start",
		fmt.Sprintf("--request-id=%s", e.requestID),
		fmt.Sprintf("--root-dag-name=%s", c.RootDAG().Name),
		fmt.Sprintf("--root-request-id=%s", c.RootDAG().RequestID),
		"--quiet",
		e.dag.Location,
	}

	if e.params != "" {
		args = append(args, "--")
		args = append(args, e.params)
	}

	logger.Debug(ctx, "Sub-DAG execution", "args", args)

	cmd := exec.CommandContext(ctx, executable, args...) // nolint:gosec
	cmd.Dir = e.workDir
	cmd.Env = append(cmd.Env, c.AllEnvs()...)
	if e.stdout != nil {
		cmd.Stdout = e.stdout
	}
	if e.stderr != nil {
		// TODO: Separate stderr and stdout for sub-DAG to avoid mixing logger output
		cmd.Stderr = e.stderr
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	e.cmd = cmd

	err = cmd.Start()
	e.lock.Unlock()

	if err != nil {
		return fmt.Errorf("failed to start sub-DAG: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("sub-DAG exited with error: %w", err)
	}

	// get results from the sub-DAG
	result, err := digraph.GetSubResult(ctx, e.requestID)
	if err != nil {
		return fmt.Errorf("failed to collect result: %w", err)
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

func (e *subDAG) SetRequestID(id string) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.requestID = id
}

func (e *subDAG) SetStdout(out io.Writer) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.stdout = out
}

func (e *subDAG) SetStderr(out io.Writer) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.stderr = out
}

func (e *subDAG) Kill(sig os.Signal) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.cmd == nil || e.cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-e.cmd.Process.Pid, sig.(syscall.Signal))
}

func init() {
	Register(digraph.ExecutorTypeSubLegacy, newSubDAG)
	Register(digraph.ExecutorTypeSub, newSubDAG)
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
