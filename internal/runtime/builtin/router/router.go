package router

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime/executor"
)

// Ensure routerExecutor implements required interfaces
var (
	_ executor.Executor              = (*routerExecutor)(nil)
	_ executor.NodeStatusDeterminer  = (*routerExecutor)(nil)
	_ executor.RouterResultProvider  = (*routerExecutor)(nil)
)

// routerExecutor is a specialized executor for router steps that perform conditional branching.
// Router steps evaluate patterns and activate downstream steps without executing commands.
type routerExecutor struct {
	stdout io.Writer
	stderr io.Writer
	step   core.Step
	result *exec.RouterResult // Stores the result after evaluation
}

// newRouter creates a new router executor.
func newRouter(_ context.Context, step core.Step) (executor.Executor, error) {
	// Validate that this step has a router configuration
	if step.Router == nil {
		return nil, fmt.Errorf("step %q is not configured as a router step", step.Name)
	}

	// Validate that router patterns have been compiled
	if err := step.Router.Validate(); err != nil {
		return nil, fmt.Errorf("router validation failed for step %q: %w", step.Name, err)
	}

	return &routerExecutor{
		stdout: os.Stdout,
		stderr: os.Stderr,
		step:   step,
		result: nil, // Will be populated during Run()
	}, nil
}

// SetStdout sets the standard output writer for the executor.
func (e *routerExecutor) SetStdout(out io.Writer) {
	e.stdout = out
}

// SetStderr sets the standard error writer for the executor.
func (e *routerExecutor) SetStderr(out io.Writer) {
	e.stderr = out
}

// Kill is a no-op for router executors as they don't spawn long-running processes.
func (*routerExecutor) Kill(_ os.Signal) error {
	return nil
}

// Run evaluates the router patterns and records which steps should be activated.
// This method is thread-safe and evaluates patterns using the pre-compiled configuration.
func (e *routerExecutor) Run(ctx context.Context) error {
	// Output status message
	_, _ = fmt.Fprintln(e.stdout, "Evaluating router patterns...")

	// TODO: Get the value to evaluate from context or previous step output
	// For now, we'll use a placeholder. This needs to be integrated with
	// the runner's context to get the actual value (e.g., from previous step's
	// stdout, exit code, or an evaluated expression).
	value := ""
	exitCode := 0

	// Evaluate the router patterns
	matchedPatterns, activatedSteps, err := e.step.Router.EvaluateRoutes(value, exitCode)
	if err != nil {
		_, _ = fmt.Fprintf(e.stderr, "Router evaluation failed: %v\n", err)
		return fmt.Errorf("router evaluation failed: %w", err)
	}

	// Store the result for retrieval by the runner
	e.result = &exec.RouterResult{
		EvaluatedValue:  value,
		EvaluatedAt:     time.Now().Format(time.RFC3339),
		MatchedPatterns: matchedPatterns,
		ActivatedSteps:  activatedSteps,
	}

	// Output the result
	_, _ = fmt.Fprintf(e.stdout, "Evaluated value: %q\n", value)
	if len(matchedPatterns) > 0 {
		_, _ = fmt.Fprintf(e.stdout, "Matched patterns: %v\n", matchedPatterns)
	} else {
		_, _ = fmt.Fprintln(e.stdout, "No patterns matched (using default)")
	}
	_, _ = fmt.Fprintf(e.stdout, "Activated steps: %v\n", activatedSteps)

	return nil
}

// DetermineNodeStatus returns NodeSucceeded since router evaluation itself succeeded.
// The actual step activation is handled by the runner based on the router result.
func (e *routerExecutor) DetermineNodeStatus() (core.NodeStatus, error) {
	return core.NodeSucceeded, nil
}

// GetRouterResult returns the router evaluation result.
// This is called by the runner after execution to retrieve the routing decision.
func (e *routerExecutor) GetRouterResult() *exec.RouterResult {
	if e.result == nil {
		return nil
	}

	// Return a deep copy to prevent external mutation
	result := &exec.RouterResult{
		EvaluatedValue:  e.result.EvaluatedValue,
		EvaluatedAt:     e.result.EvaluatedAt,
		MatchedPatterns: make([]string, len(e.result.MatchedPatterns)),
		ActivatedSteps:  make([]string, len(e.result.ActivatedSteps)),
	}
	copy(result.MatchedPatterns, e.result.MatchedPatterns)
	copy(result.ActivatedSteps, e.result.ActivatedSteps)

	return result
}

// init registers the router executor.
func init() {
	executor.RegisterExecutor("router", newRouter, nil, core.ExecutorCapabilities{})
}
