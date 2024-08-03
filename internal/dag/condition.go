// Copyright (C) 2024 The Daguflow/Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
