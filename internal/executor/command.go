package executor

import (
	"context"
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/dagu-dev/dagu/internal/dag"
)

type CommandExecutor struct {
	cmd *exec.Cmd
}

func (e *CommandExecutor) Run() error {
	return e.cmd.Run()
}

func (e *CommandExecutor) SetStdout(out io.Writer) {
	e.cmd.Stdout = out
}

func (e *CommandExecutor) SetStderr(out io.Writer) {
	e.cmd.Stderr = out
}

func (e *CommandExecutor) Kill(sig os.Signal) error {
	if e.cmd == nil || e.cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-e.cmd.Process.Pid, sig.(syscall.Signal))
}

func CreateCommandExecutor(ctx context.Context, step *dag.Step) (Executor, error) {
	cmd := exec.CommandContext(ctx, step.Command, step.Args...)
	cmd.Dir = step.Dir
	cmd.Env = append(cmd.Env, step.Variables...)
	step.OutputVariables.Range(func(key, value interface{}) bool {
		cmd.Env = append(cmd.Env, value.(string))
		return true
	})
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	return &CommandExecutor{
		cmd: cmd,
	}, nil
}

func init() {
	Register("", CreateCommandExecutor)
	Register("command", CreateCommandExecutor)
}
