package dag

import (
	"errors"
	"fmt"

	"github.com/dagu-dev/dagu/internal/utils"
)

// Condition represents a condition to be evaluated by the agent.
type Condition struct {
	Condition string
	Expected  string
}

var (
	errConditionNotMet = errors.New("condition was not met")
	errEvalCondition   = errors.New("failed to evaluate condition")
)

// Evaluate returns the actual value of the condition.
func (c *Condition) Evaluate() (string, error) {
	return utils.ParseVariable(c.Condition)
}

// CheckResult checks if the actual value of the condition matches the expected value.
func (c *Condition) CheckResult(actualValue string) error {
	if c.Expected != actualValue {
		return fmt.Errorf("%w. Condition=%s Expected=%s Actual=%s", errConditionNotMet, c.Condition, c.Expected, actualValue)
	}
	return nil
}

// EvalCondition evaluates a single condition and checks the result.
func EvalCondition(c *Condition) error {
	actual, err := c.Evaluate()
	if err != nil {
		return fmt.Errorf("%w. Condition=%s Error=%v", errEvalCondition, c.Condition, err)
	}
	return c.CheckResult(actual)
}

// EvalConditions evaluates a list of conditions and checks the results.
func EvalConditions(cond []*Condition) error {
	for _, c := range cond {
		err := EvalCondition(c)
		if err != nil {
			return err
		}
	}
	return nil
}
