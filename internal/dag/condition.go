package dag

import (
	"errors"
	"fmt"
	"os"
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
func (c Condition) eval() (string, error) {
	return substituteCommands(os.ExpandEnv(c.Condition))
}

var (
	errConditionNotMet = errors.New("condition was not met")
	errEvalCondition   = errors.New("failed to evaluate condition")
)

// evalCondition evaluates a single condition and checks the result.
// It returns an error if the condition was not met.
func evalCondition(c Condition) error {
	actual, err := c.eval()
	if err != nil {
		return fmt.Errorf(
			"%w. Condition=%s Error=%v", errEvalCondition, c.Condition, err,
		)
	}

	if c.Expected != actual {
		return fmt.Errorf(
			"%w. Condition=%s Expected=%s Actual=%s",
			errConditionNotMet,
			c.Condition,
			c.Expected,
			actual,
		)
	}

	return nil
}

// EvalConditions evaluates a list of conditions and checks the results.
// It returns an error if any of the conditions were not met.
func EvalConditions(cond []Condition) error {
	for _, c := range cond {
		if err := evalCondition(c); err != nil {
			return err
		}
	}

	return nil
}
