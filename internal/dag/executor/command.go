// Copyright (C) 2024 The Daguflow/Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/daguflow/dagu/internal/dag"
	"github.com/daguflow/dagu/internal/util"
)

type commandExecutor struct {
	cmd  *exec.Cmd
	lock sync.Mutex
}

func newCommand(ctx context.Context, step dag.Step) (Executor, error) {
	// nolint: gosec
	cmd := exec.CommandContext(ctx, step.Command, step.Args...)
	if len(step.Dir) > 0 && !util.FileExists(step.Dir) {
		return nil, fmt.Errorf("directory %q does not exist", step.Dir)
	}
	dagContext, err := dag.GetContext(ctx)
	if err != nil {
		return nil, err
	}

	cmd.Dir = step.Dir
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Env = append(cmd.Env, step.Variables...)
	cmd.Env = append(cmd.Env, dagContext.Envs...)
	step.OutputVariables.Range(func(_, value any) bool {
		cmd.Env = append(cmd.Env, value.(string))
		return true
	})
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	return &commandExecutor{
		cmd: cmd,
	}, nil
}

func (e *commandExecutor) Run() error {
	e.lock.Lock()
	err := e.cmd.Start()
	e.lock.Unlock()
	if err != nil {
		return err
	}
	return e.cmd.Wait()
}

func (e *commandExecutor) SetStdout(out io.Writer) {
	e.cmd.Stdout = out
}

func (e *commandExecutor) SetStderr(out io.Writer) {
	e.cmd.Stderr = out
}

func (e *commandExecutor) Kill(sig os.Signal) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.cmd == nil || e.cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-e.cmd.Process.Pid, sig.(syscall.Signal))
}

func init() {
	Register("", newCommand)
	Register("command", newCommand)
}
