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
func EvalConditions(ctx context.Context, shell []string, cond []*core.Condition) error {
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
// If c.Negate is true, the result is inverted: the condition passes when it
// would normally fail, and vice versa.
func EvalCondition(ctx context.Context, shell []string, c *core.Condition) error {
	var err error
	switch {
	case c.Condition != "" && c.Expected != "":
		err = matchCondition(ctx, c)

	default:
		err = evalCommand(ctx, shell, c)
	}

	// Apply negation if needed
	if c.Negate {
		if err == nil {
			return fmt.Errorf("%w: condition matched but negate is true", ErrConditionNotMet)
		}
		// Condition failed as expected when negated, so it's a success
		return nil
	}

	return err
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
	if rCtx := GetDAGContext(ctx); rCtx.DAG != nil && rCtx.DAG.MaxOutputSize > 0 {
		maxOutputSize = rCtx.DAG.MaxOutputSize
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

func evalCommand(ctx context.Context, shell []string, c *core.Condition) error {
	commandToRun, err := EvalString(ctx, c.Condition, cmdutil.OnlyReplaceVars())
	if err != nil {
		return fmt.Errorf("failed to evaluate command: %w", err)
	}
	if len(shell) > 0 {
		return runShellCommand(ctx, shell, commandToRun)
	}
	return runDirectCommand(ctx, commandToRun)
}

func runShellCommand(ctx context.Context, shell []string, commandToRun string) error {
	args := append(shell[1:], "-c", commandToRun)
	cmd := exec.CommandContext(ctx, shell[0], args...) // nolint:gosec
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
