package router

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime/executor"
)

var _ executor.Executor = (*routerExecutor)(nil)

type routerExecutor struct {
	stdout io.Writer
	step   core.Step
}

func newRouter(_ context.Context, step core.Step) (executor.Executor, error) {
	return &routerExecutor{
		stdout: os.Stdout,
		step:   step,
	}, nil
}

func (e *routerExecutor) SetStdout(out io.Writer) { e.stdout = out }
func (e *routerExecutor) SetStderr(_ io.Writer)   {}
func (*routerExecutor) Kill(_ os.Signal) error    { return nil }

func (e *routerExecutor) Run(_ context.Context) error {
	if e.step.Router != nil {
		_, _ = fmt.Fprintf(e.stdout, "Router evaluating: %s\n", e.step.Router.Value)
		for _, route := range e.step.Router.Routes {
			_, _ = fmt.Fprintf(e.stdout, "  %s -> %v\n", route.Pattern, route.Targets)
		}
	}
	return nil
}

func init() {
	executor.RegisterExecutor("router", newRouter, nil, core.ExecutorCapabilities{})
}
