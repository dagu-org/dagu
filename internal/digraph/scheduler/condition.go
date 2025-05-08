package scheduler

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/stringutil"
)

// Errors for condition evaluation
var (
	ErrConditionNotMet = fmt.Errorf("condition was not met")
)

// EvalConditions evaluates a list of conditions and checks the results.
// It returns an error if any of the conditions were not met.
func EvalConditions(ctx context.Context, cond []digraph.Condition) error {
	for _, c := range cond {
		if err := evalCondition(ctx, c); err != nil {
			return err
		}
	}

	return nil
}

// eval evaluates the condition and returns the actual value.
// It returns an error if the evaluation failed or the condition is invalid.
func eval(ctx context.Context, c digraph.Condition) (bool, error) {
	switch {
	case c.Condition != "":
		return matchCondition(ctx, c)

	case c.Command != "":
		return evalCommand(ctx, c)

	default:
		return false, fmt.Errorf("invalid condition: Condition=%s", c.Condition)
	}
}

// evalCondition evaluates a single condition and checks the result.
// It returns an error if the condition was not met.
func evalCondition(ctx context.Context, c digraph.Condition) error {
	matched, err := eval(ctx, c)
	if err != nil {
		if errors.Is(err, ErrConditionNotMet) {
			return err
		}
		return fmt.Errorf("failed to evaluate condition: Condition=%s Error=%v", c.Condition, err)
	}

	if !matched {
		return fmt.Errorf("%w: Condition=%s Expected=%s", ErrConditionNotMet, c.Condition, c.Expected)
	}

	// Condition was met
	return nil
}

func evalCommand(ctx context.Context, c digraph.Condition) (bool, error) {
	commandToRun, err := digraph.EvalString(ctx, c.Command, cmdutil.OnlyReplaceVars())
	if err != nil {
		return false, fmt.Errorf("failed to evaluate command: %w", err)
	}
	if shell := cmdutil.GetShellCommand(""); shell != "" {
		return runShellCommand(ctx, shell, commandToRun)
	}
	return runDirectCommand(ctx, commandToRun)
}

func runShellCommand(ctx context.Context, shell, commandToRun string) (bool, error) {
	cmd := exec.CommandContext(ctx, shell, "-c", commandToRun)
	_, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("%w: %s", ErrConditionNotMet, err)
	}
	return true, nil
}

func runDirectCommand(ctx context.Context, commandToRun string) (bool, error) {
	cmd := exec.CommandContext(ctx, commandToRun)
	_, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("%w: %s", ErrConditionNotMet, err)
	}
	return true, nil
}

func matchCondition(ctx context.Context, c digraph.Condition) (bool, error) {
	evaluatedVal, err := digraph.EvalString(ctx, c.Condition)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate condition: Condition=%s Error=%v", c.Condition, err)
	}
	if stringutil.MatchPattern(ctx, evaluatedVal, []string{c.Expected}, stringutil.WithExactMatch()) {
		return true, nil
	}
	return false, fmt.Errorf("%w: Condition=%s Expected=%s", ErrConditionNotMet, c.Condition, c.Expected)
}
