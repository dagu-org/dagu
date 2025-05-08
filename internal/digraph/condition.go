package digraph

import (
	"fmt"
)

// Condition contains a condition and the expected value.
// Conditions are evaluated and compared to the expected value.
// The condition can be a command substitution or an environment variable.
// The expected value must be a string without any substitutions.
type Condition struct {
	Command   string `json:"command,omitempty"`   // Command to evaluate
	Condition string `json:"condition,omitempty"` // Condition to evaluate
	Expected  string `json:"expected,omitempty"`  // Expected value
}

func (c Condition) Validate() error {
	switch {
	case c.Condition != "":
		if c.Expected == "" {
			return fmt.Errorf("expected value is required for condition: Condition=%s", c.Condition)
		}

	case c.Command != "":
		// Command is required
	default:
		return fmt.Errorf("invalid condition: Condition=%s", c.Condition)
	}

	return nil
}
