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
	Command   string `json:"Command,omitempty"`   // Command to evaluate
	Condition string `json:"Condition,omitempty"` // Condition to evaluate
	Expected  string `json:"Expected,omitempty"`  // Expected value
}

// eval evaluates the condition and returns the actual value.
// It returns an error if the evaluation failed or the condition is invalid.
func (c Condition) eval(ctx context.Context) (bool, error) {
	switch {
	case c.Condition != "":
		return c.evalCondition(ctx)

	default:
		return false, fmt.Errorf("invalid condition: Condition=%s", c.Condition)
	}
}

// func (c Condition) evalCommand(ctx context.Context) (bool, error) {
// 	command, err := GetContext(ctx).EvalString(c.Command)
// 	if err !=nil {
// 		return false, err
// 	}
// 	// Run the command and get the exit code
// 	exitCode, err := cmdutil.WithVariables()
// }

func (c Condition) evalCondition(ctx context.Context) (bool, error) {
	if IsStepContext(ctx) {
		evaluatedVal, err := GetStepContext(ctx).EvalString(c.Condition)
		if err != nil {
			return false, err
		}
		return c.Expected == evaluatedVal, nil
	}

	evaluatedVal, err := GetContext(ctx).EvalString(c.Condition)
	if err != nil {
		return false, err
	}

	return c.Expected == evaluatedVal, nil
}

func (c Condition) String() string {
	return fmt.Sprintf("Condition=%s Expected=%s", c.Condition, c.Expected)
}

// evalCondition evaluates a single condition and checks the result.
// It returns an error if the condition was not met.
func evalCondition(ctx context.Context, c Condition) error {
	matched, err := c.eval(ctx)
	if err != nil {
		return fmt.Errorf("failed to evaluate condition: Condition=%s Error=%v", c.Condition, err)
	}

	if !matched {
		return fmt.Errorf("error condition was not met: Condition=%s Expected=%s", c.Condition, c.Expected)
	}

	// Condition was met
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
