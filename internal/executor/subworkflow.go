package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/dagu-dev/dagu/internal/util"

	"github.com/dagu-dev/dagu/internal/dag"
)

type SubWorkflowExecutor struct {
	cmd  *exec.Cmd
	lock sync.Mutex
}

func (e *SubWorkflowExecutor) Run() error {
	e.lock.Lock()
	err := e.cmd.Start()
	e.lock.Unlock()
	if err != nil {
		return err
	}
	return e.cmd.Wait()
}

func (e *SubWorkflowExecutor) SetStdout(out io.Writer) {
	e.cmd.Stdout = out
}

func (e *SubWorkflowExecutor) SetStderr(out io.Writer) {
	e.cmd.Stderr = out
}

func (e *SubWorkflowExecutor) Kill(sig os.Signal) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.cmd == nil || e.cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-e.cmd.Process.Pid, sig.(syscall.Signal))
}

func CreateSubWorkflowExecutor(ctx context.Context, step dag.Step) (Executor, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	dagCtx, err := dag.GetContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get dag context: %w", err)
	}

	sugDAG, err := dagCtx.Finder.Find(step.SubWorkflow.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find subworkflow %q: %w", step.SubWorkflow.Name, err)
	}

	params := os.ExpandEnv(step.SubWorkflow.Params)

	args := []string{
		"start",
		fmt.Sprintf("--params=%q", params),
		sugDAG.Location,
	}

	cmd := exec.CommandContext(ctx, executable, args...)
	if len(step.Dir) > 0 && !util.FileExists(step.Dir) {
		return nil, fmt.Errorf("directory %q does not exist", step.Dir)
	}
	cmd.Dir = step.Dir
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Env = append(cmd.Env, step.Variables...)
	step.OutputVariables.Range(func(key, value interface{}) bool {
		cmd.Env = append(cmd.Env, value.(string))
		return true
	})
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	return &SubWorkflowExecutor{
		cmd: cmd,
	}, nil
}

func init() {
	Register(dag.ExecutorTypeSubWorkflow, CreateSubWorkflowExecutor)
}
