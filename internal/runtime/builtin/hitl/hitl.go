package hitl

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime/executor"
)

var (
	_ executor.Executor             = (*hitlExecutor)(nil)
	_ executor.NodeStatusDeterminer = (*hitlExecutor)(nil)
)

type hitlExecutor struct {
	stdout io.Writer
	stderr io.Writer
	step   core.Step
	config Config
}

func newHITL(_ context.Context, step core.Step) (executor.Executor, error) {
	var cfg Config
	if step.ExecutorConfig.Config != nil {
		if err := decodeConfig(step.ExecutorConfig.Config, &cfg); err != nil {
			return nil, fmt.Errorf("failed to decode hitl config: %w", err)
		}
	}

	return &hitlExecutor{
		stdout: os.Stdout,
		stderr: os.Stderr,
		step:   step,
		config: cfg,
	}, nil
}

func (e *hitlExecutor) SetStdout(out io.Writer) {
	e.stdout = out
}

func (e *hitlExecutor) SetStderr(out io.Writer) {
	e.stderr = out
}

func (*hitlExecutor) Kill(_ os.Signal) error {
	return nil
}

// Run outputs the HITL step information and completes immediately.
// The actual waiting is handled by the runner which detects NodeWaiting status.
func (e *hitlExecutor) Run(_ context.Context) error {
	_, _ = fmt.Fprintln(e.stdout, "Waiting for human approval...")

	if e.config.Prompt != "" {
		_, _ = fmt.Fprintln(e.stdout)
		_, _ = fmt.Fprintln(e.stdout, "Prompt:")
		_, _ = fmt.Fprintln(e.stdout, e.config.Prompt)
	}

	if len(e.config.Input) > 0 {
		_, _ = fmt.Fprintln(e.stdout)
		_, _ = fmt.Fprintf(e.stdout, "Expected inputs: %v\n", e.config.Input)
		if len(e.config.Required) > 0 {
			_, _ = fmt.Fprintf(e.stdout, "Required inputs: %v\n", e.config.Required)
		}
	}

	return nil
}

// DetermineNodeStatus returns NodeWaiting to signal this step requires approval.
func (e *hitlExecutor) DetermineNodeStatus() (core.NodeStatus, error) {
	return core.NodeWaiting, nil
}

func validateConfig(step core.Step) error {
	var cfg Config
	if step.ExecutorConfig.Config != nil {
		if err := decodeConfig(step.ExecutorConfig.Config, &cfg); err != nil {
			return fmt.Errorf("failed to decode hitl config: %w", err)
		}
	}

	// Validate that all required fields are in input list
	for _, req := range cfg.Required {
		if !slices.Contains(cfg.Input, req) {
			return fmt.Errorf("required field %q is not in input list", req)
		}
	}

	return nil
}

func init() {
	executor.RegisterExecutor("hitl", newHITL, validateConfig, core.ExecutorCapabilities{})
}
