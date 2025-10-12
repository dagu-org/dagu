package runtime

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/common/stringutil"
	"github.com/dagu-org/dagu/internal/core"
)

// Errors for condition evaluation
var (
	ErrConditionNotMet = fmt.Errorf("condition was not met")
)

// Error message for the case not all condition was not met
const ErrMsgOtherConditionNotMet = "other condition was not met"

// EvalConditions evaluates a list of conditions and checks the results.
// It returns an error if any of the conditions were not met.
func EvalConditions(ctx context.Context, shell string, cond []*core.Condition) error {
	var lastErr error

	for i := range cond {
		if err := EvalCondition(ctx, shell, cond[i]); err != nil {
			cond[i].SetErrorMessage(err.Error())
			lastErr = err
		}
	}

	if lastErr != nil {
		// Set error message
		for i := range cond {
			if cond[i].GetErrorMessage() != "" {
				continue
			}
			cond[i].SetErrorMessage(ErrMsgOtherConditionNotMet)
		}
	}

	return lastErr
}

// EvalCondition evaluates the condition and returns the actual value.
// It returns an error if the evaluation failed or the condition is invalid.
func EvalCondition(ctx context.Context, shell string, c *core.Condition) error {
	switch {
	case c.Condition != "" && c.Expected != "":
		return matchCondition(ctx, c)

	default:
		return evalCommand(ctx, shell, c)
	}
}

// matchCondition evaluates the condition and checks if it matches the expected value.
// It returns an error if the condition was not met.
func matchCondition(ctx context.Context, c *core.Condition) error {
	evaluatedVal, err := EvalString(ctx, c.Condition)
	if err != nil {
		return fmt.Errorf("failed to evaluate the value: Error=%v", err)
	}

	// Get maxOutputSize from DAG configuration
	var maxOutputSize = 1024 * 1024 // Default 1MB
	if env := core.GetDAGContextFromContext(ctx); env.DAG != nil && env.DAG.MaxOutputSize > 0 {
		maxOutputSize = env.DAG.MaxOutputSize
	}

	matchOpts := []stringutil.MatchOption{
		stringutil.WithExactMatch(),
		stringutil.WithMaxBufferSize(maxOutputSize),
	}

	if stringutil.MatchPattern(ctx, evaluatedVal, []string{c.Expected}, matchOpts...) {
		return nil
	}
	// Return an helpful error message if the condition is not met
	return fmt.Errorf("%w: expected %q, got %q", ErrConditionNotMet, c.Expected, evaluatedVal)
}

func evalCommand(ctx context.Context, shell string, c *core.Condition) error {
	commandToRun, err := EvalString(ctx, c.Condition, cmdutil.OnlyReplaceVars())
	if err != nil {
		return fmt.Errorf("failed to evaluate command: %w", err)
	}
	if shell != "" {
		return runShellCommand(ctx, shell, commandToRun)
	}
	return runDirectCommand(ctx, commandToRun)
}

func runShellCommand(ctx context.Context, shell, commandToRun string) error {
	cmd := exec.CommandContext(ctx, shell, "-c", commandToRun)
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("%w: %s", ErrConditionNotMet, err)
	}
	return nil
}

func runDirectCommand(ctx context.Context, commandToRun string) error {
	cmd := exec.CommandContext(ctx, commandToRun)
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("%w: %s", ErrConditionNotMet, err)
	}
	return nil
}
