package digraph

import (
	"fmt"
)

// Condition contains a condition and the expected value.
// Conditions are evaluated and compared to the expected value.
// The condition can be a command substitution or an environment variable.
// The expected value must be a string without any substitutions.
type Condition struct {
	Condition string `json:"condition,omitempty"` // Condition to evaluate
	Expected  string `json:"expected,omitempty"`  // Expected value
}

func (c Condition) Validate() error {
	if c.Condition == "" {
		return fmt.Errorf("condition is required")
	}
	return nil
}
