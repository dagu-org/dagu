package core

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Condition contains a condition and the expected value.
// Conditions are evaluated and compared to the expected value.
// The condition can be a command substitution or an environment variable.
// The expected value must be a string without any substitutions.
type Condition struct {
	mu sync.Mutex

	Condition    string // Condition to evaluate
	Expected     string // Expected value
	Negate       bool   // Negate the condition result (run when condition does NOT match)
	errorMessage string // Error message if the condition is not met
}

func (c *Condition) MarshalJSON() ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return json.Marshal(struct {
		Condition    string `json:"condition,omitempty"`
		Expected     string `json:"expected,omitempty"`
		Negate       bool   `json:"negate,omitempty"`
		ErrorMessage string `json:"error,omitempty"`
	}{
		Condition:    c.Condition,
		Expected:     c.Expected,
		Negate:       c.Negate,
		ErrorMessage: c.errorMessage,
	})
}

func (c *Condition) UnmarshalJSON(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var tmp struct {
		Condition    string `json:"condition,omitempty"`
		Expected     string `json:"expected,omitempty"`
		Negate       bool   `json:"negate,omitempty"`
		ErrorMessage string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	c.Condition = tmp.Condition
	c.Expected = tmp.Expected
	c.Negate = tmp.Negate
	c.errorMessage = tmp.ErrorMessage
	return nil
}

func (c *Condition) Validate() error {
	if c.Condition == "" {
		return fmt.Errorf("condition is required")
	}
	return nil
}

func (c *Condition) SetErrorMessage(msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.errorMessage = msg
}

func (c *Condition) GetErrorMessage() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.errorMessage
}
