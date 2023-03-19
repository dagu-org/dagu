package dag

import (
	"fmt"

	"github.com/yohamta/dagu/internal/utils"
)

// Condition represents a condition to be evaluated by the agent.
type Condition struct {
	Condition string
	Expected  string
}

// Evaluate returns the actual value of the condition.
func (c *Condition) Evaluate() (string, error) {
	return utils.ParseVariable(c.Condition)
}

// CheckResult checks if the actual value of the condition matches the expected value.
func (c *Condition) CheckResult(actualValue string) error {
	if c.Expected != actualValue {
		return fmt.Errorf("condition was not met. Condition=%s Expected=%s Actual=%s", c.Condition, c.Expected, actualValue)
	}
	return nil
}

// EvalCondition evaluates a single condition and checks the result.
func EvalCondition(c *Condition) error {
	actual, err := c.Evaluate()
	if err != nil {
		return fmt.Errorf("failed to evaluate condition. Condition=%s Error=%v", c.Condition, err)
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
