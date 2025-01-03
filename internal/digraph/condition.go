package digraph

import (
	"context"
	"fmt"
)

// Condition contains a condition and the expected value.
// Conditions are evaluated and compared to the expected value.
// The condition can be a command substitution or an environment variable.
// The expected value must be a string without any substitutions.
type Condition struct {
	Condition string // Condition to evaluate
	Expected  string // Expected value
}

// eval evaluates the condition and returns the actual value.
// It returns an error if the evaluation failed or the condition is invalid.
func (c Condition) eval(ctx context.Context) (string, error) {
	if IsStepContext(ctx) {
		return GetStepContext(ctx).EvalString(c.Condition)
	}

	return GetContext(ctx).EvalString(c.Condition)
}

// evalCondition evaluates a single condition and checks the result.
// It returns an error if the condition was not met.
func evalCondition(ctx context.Context, c Condition) error {
	actual, err := c.eval(ctx)
	if err != nil {
		return fmt.Errorf("failed to evaluate condition: Condition=%s Error=%v", c.Condition, err)
	}

	if c.Expected != actual {
		return fmt.Errorf("error condition was not met: Condition=%s Expected=%s Actual=%s", c.Condition, c.Expected, actual)
	}

	return nil
}

// EvalConditions evaluates a list of conditions and checks the results.
// It returns an error if any of the conditions were not met.
func EvalConditions(ctx context.Context, cond []Condition) error {
	for _, c := range cond {
		if err := evalCondition(ctx, c); err != nil {
			return err
		}
	}

	return nil
}
