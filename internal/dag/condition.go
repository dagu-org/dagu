// Copyright (C) 2024 The Dagu Authors
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
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Condition contains a condition and the expected value.
// Conditions are evaluated and compared to the expected value.
// The condition can be a command substitution or an environment variable.
// The expected value must be a string without any substitutions.
type Condition struct {
	Condition string // Condition to evaluate
	Expected  string // Expected value
}

// // eval evaluates the condition and returns the actual value.
// // It returns an error if the evaluation failed or the condition is invalid.
// func (c Condition) eval() (string, error) {
// 	return substituteCommands(os.ExpandEnv(c.Condition))
// }

var (
	errConditionNotMet = errors.New("condition was not met")
	errEvalCondition   = errors.New("failed to evaluate condition")
)

// eval function to evaluate the condition remains the same
func (c Condition) eval() (string, error) {
	// Expand environment variables in the condition
	expandedCondition := os.ExpandEnv(c.Condition)
	log.Print("condition: ", c, expandedCondition)

	// Check if it's a Lisp-like condition and evaluate if so
	if isLispCondition(expandedCondition) {
		result, err := evalLisp(expandedCondition)
		log.Print("result: ", result)
		if err != nil {
			return "", fmt.Errorf("Lisp evaluation error: %w", err)
		}
		return strconv.FormatBool(result), nil
	}

	// Otherwise, default to simple command substitution
	return substituteCommands(expandedCondition)
}

// isLispCondition checks if the condition follows a Lisp-like syntax
func isLispCondition(condition string) bool {
	return strings.HasPrefix(condition, "(") && strings.HasSuffix(condition, ")")
}

// evalLisp interprets a simple Lisp-like expression with basic operators
func evalLisp(expr string) (bool, error) {
	tokens := tokenize(expr)

	log.Print("tokens: ", tokens, " - ", len(tokens))
	if len(tokens) == 0 {
		return false, fmt.Errorf("empty expression")
	}

	switch tokens[0] {
	case "or", "OR":
		return evalOr(tokens[1:])
	case "and", "AND":
		return evalAnd(tokens[1:])
	case "eq", "==":
		return evalEq(tokens[1:])
	case "ne", "!=":
		return evalNe(tokens[1:])
	case "gt", ">":
		return evalGt(tokens[1:])
	case "ge", ">=":
		return evalGe(tokens[1:])
	case "lt", "<":
		return evalLt(tokens[1:])
	case "le", "<=":
		return evalLe(tokens[1:])
	default:
		return false, fmt.Errorf("unsupported operator: %s", tokens[0])
	}
}

// tokenize splits the Lisp expression into tokens
func tokenize(input string) []string {
	if input[0] == '(' && input[len(input)-1] == ')' {
		input = input[1 : len(input)-1]
	}

	// Split the expression by spaces to get the main operator
	parts := strings.Fields(input)
	mainOperator := parts[0]

	// If no nested expressions, return simple expression
	if len(parts) == 3 {
		return []string{mainOperator, parts[1], parts[2]}
	}

	// For expressions with nested parts, capture each one
	re := regexp.MustCompile(`\(([^()]+)\)|\S+`)
	matches := re.FindAllString(input, -1)

	// Build the result list starting with the main operator
	var result []string
	result = append(result, mainOperator)
	for _, match := range matches[1:] {
		// Recursively parse each nested expression
		result = append(result, strings.TrimSpace(match))
	}
	return result
}

// evalOr evaluates an "or" expression
func evalOr(args []string) (bool, error) {
	for _, arg := range args {
		res, err := evalLisp(arg)
		log.Print("", res)
		if err != nil {
			return false, err
		}
		if res {
			return true, nil
		}
	}
	return false, nil
}

// evalAnd evaluates an "and" expression
func evalAnd(args []string) (bool, error) {
	for _, arg := range args {
		res, err := evalLisp(arg)
		if err != nil {
			return false, err
		}
		if !res {
			return false, nil
		}
	}
	return true, nil
}

// Comparison function helpers
func evalEq(args []string) (bool, error) { return compare(args, func(a, b int) bool { return a == b }) }
func evalNe(args []string) (bool, error) { return compare(args, func(a, b int) bool { return a != b }) }
func evalGt(args []string) (bool, error) { return compare(args, func(a, b int) bool { return a > b }) }
func evalGe(args []string) (bool, error) { return compare(args, func(a, b int) bool { return a >= b }) }
func evalLt(args []string) (bool, error) { return compare(args, func(a, b int) bool { return a < b }) }
func evalLe(args []string) (bool, error) { return compare(args, func(a, b int) bool { return a <= b }) }

// compare evaluates numeric comparisons using a custom comparison function
func compare(args []string, cmpFunc func(int, int) bool) (bool, error) {
	if len(args) != 2 {
		return false, fmt.Errorf("comparison expects exactly two arguments")
	}

	aVal, bVal := args[0], args[1]

	// Check if aVal is an environment variable and get its value
	aEnvVal := os.Getenv(aVal)
	if aEnvVal != "" {
		aVal = aEnvVal
	}

	// Parse both arguments as integers
	aInt, aErr := strconv.Atoi(aVal)
	bInt, bErr := strconv.Atoi(bVal)

	if aErr == nil && bErr == nil {
		return cmpFunc(aInt, bInt), nil
	}

	return aVal == bVal, nil
}

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
