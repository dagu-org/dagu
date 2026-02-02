package core

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// RouterConfig defines routing behavior for a step
type RouterConfig struct {
	Value   string              `json:"value"`   // Expression to evaluate
	Mode    RouterMode          `json:"mode"`    // exclusive or multi-select
	Routes  map[string][]string `json:"routes"`  // pattern -> step names
	Default []string            `json:"default"` // fallback step names

	// Internal: compiled patterns (not serialized)
	compiledPatterns map[string]*CompiledPattern `json:"-"`
}

// RouterMode defines how routes are selected
type RouterMode string

const (
	RouterModeExclusive   RouterMode = "exclusive"    // First match only
	RouterModeMultiSelect RouterMode = "multi-select" // All matches
)

// CompiledPattern holds pre-compiled pattern matcher
type CompiledPattern struct {
	Original     string
	IsRegex      bool
	IsArray      bool
	IsExpression bool
	Regex        *regexp.Regexp
	Values       []string
}

// Validate checks router configuration with security measures
func (r *RouterConfig) Validate() error {
	if r.Value == "" {
		return fmt.Errorf("router.value is required")
	}

	// DoS prevention: limit value expression length
	if len(r.Value) > 4096 {
		return fmt.Errorf("router.value exceeds 4KB limit (got %d bytes)", len(r.Value))
	}

	// Validate mode
	if r.Mode == "" {
		r.Mode = RouterModeExclusive // Default
	}
	if r.Mode != RouterModeExclusive && r.Mode != RouterModeMultiSelect {
		return fmt.Errorf("router.mode must be 'exclusive' or 'multi-select', got '%s'", r.Mode)
	}

	// Must have routes or default
	if len(r.Routes) == 0 && len(r.Default) == 0 {
		return fmt.Errorf("router must have 'routes' or 'default' specified")
	}

	// Performance limit: max 1000 routes
	if len(r.Routes) > 1000 {
		return fmt.Errorf("router routes limit exceeded: %d routes (max 1000)", len(r.Routes))
	}

	// Compile and cache all patterns at build time (CRITICAL for performance)
	r.compiledPatterns = make(map[string]*CompiledPattern, len(r.Routes))
	for pattern := range r.Routes {
		compiled, err := CompilePattern(pattern)
		if err != nil {
			return fmt.Errorf("invalid route pattern '%s': %w", pattern, err)
		}
		r.compiledPatterns[pattern] = compiled
	}

	// Validate default steps are not empty
	for i, step := range r.Default {
		if strings.TrimSpace(step) == "" {
			return fmt.Errorf("router.default[%d] is empty", i)
		}
	}

	// Validate route steps are not empty
	for pattern, steps := range r.Routes {
		if len(steps) == 0 {
			return fmt.Errorf("router route '%s' has no steps", pattern)
		}
		for i, step := range steps {
			if strings.TrimSpace(step) == "" {
				return fmt.Errorf("router route '%s' step[%d] is empty", pattern, i)
			}
		}
	}

	return nil
}

// CompilePattern validates and compiles a pattern at build time
func CompilePattern(pattern string) (*CompiledPattern, error) {
	cp := &CompiledPattern{Original: pattern}

	// Regex pattern: /^regex$/
	if strings.HasPrefix(pattern, "/") && strings.HasSuffix(pattern, "/") && len(pattern) > 2 {
		regexStr := pattern[1 : len(pattern)-1]

		// ReDoS prevention: limit regex complexity
		if len(regexStr) > 1000 {
			return nil, fmt.Errorf("regex too long: %d characters (max 1000)", len(regexStr))
		}

		// Compile with timeout to prevent hangs
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		regex, err := compileRegexSafe(ctx, regexStr)
		if err != nil {
			return nil, fmt.Errorf("regex compilation failed: %w", err)
		}

		cp.IsRegex = true
		cp.Regex = regex
		return cp, nil
	}

	// Array pattern: [val1, val2, val3]
	if strings.HasPrefix(pattern, "[") && strings.HasSuffix(pattern, "]") {
		inner := pattern[1 : len(pattern)-1]
		if inner == "" {
			return nil, fmt.Errorf("empty array pattern")
		}

		parts := strings.Split(inner, ",")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				return nil, fmt.Errorf("empty value in array pattern")
			}
			cp.Values = append(cp.Values, trimmed)
		}

		if len(cp.Values) == 0 {
			return nil, fmt.Errorf("array pattern has no values")
		}

		cp.IsArray = true
		return cp, nil
	}

	// Expression pattern: contains operators like ==, &&, ||
	if containsOperators(pattern) {
		// Validate expression at build time
		if err := validateExpression(pattern); err != nil {
			return nil, fmt.Errorf("invalid expression: %w", err)
		}
		cp.IsExpression = true
		return cp, nil
	}

	// Plain string match
	return cp, nil
}

// compileRegexSafe compiles regex with timeout to prevent hangs
func compileRegexSafe(ctx context.Context, pattern string) (*regexp.Regexp, error) {
	type result struct {
		regex *regexp.Regexp
		err   error
	}

	ch := make(chan result, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- result{nil, fmt.Errorf("regex compilation panic: %v", r)}
			}
		}()

		regex, err := regexp.Compile(pattern)
		ch <- result{regex, err}
	}()

	select {
	case res := <-ch:
		return res.regex, res.err
	case <-ctx.Done():
		return nil, fmt.Errorf("regex compilation timeout after 5 seconds")
	}
}

// containsOperators checks if pattern contains expression operators
func containsOperators(s string) bool {
	return strings.Contains(s, "==") ||
		strings.Contains(s, "!=") ||
		strings.Contains(s, "&&") ||
		strings.Contains(s, "||") ||
		strings.Contains(s, ">=") ||
		strings.Contains(s, "<=") ||
		strings.Contains(s, "@value") ||
		strings.Contains(s, "@exitCode")
}

// validateExpression performs basic validation of expression syntax
func validateExpression(exprStr string) error {
	// Length limit (DoS prevention)
	if len(exprStr) > 2048 {
		return fmt.Errorf("expression too long: %d characters (max 2KB)", len(exprStr))
	}

	// Basic sanity checks
	if strings.TrimSpace(exprStr) == "" {
		return fmt.Errorf("expression is empty")
	}

	// Check for balanced quotes
	quoteCount := strings.Count(exprStr, "'")
	if quoteCount%2 != 0 {
		return fmt.Errorf("unbalanced single quotes in expression")
	}

	doubleQuoteCount := strings.Count(exprStr, "\"")
	if doubleQuoteCount%2 != 0 {
		return fmt.Errorf("unbalanced double quotes in expression")
	}

	// Check for balanced parentheses
	parenBalance := 0
	for _, ch := range exprStr {
		if ch == '(' {
			parenBalance++
		} else if ch == ')' {
			parenBalance--
		}
		if parenBalance < 0 {
			return fmt.Errorf("unbalanced parentheses in expression")
		}
	}
	if parenBalance != 0 {
		return fmt.Errorf("unbalanced parentheses in expression")
	}

	// Reject dangerous patterns (defense in depth)
	dangerous := []string{
		"system(",
		"exec(",
		"shell(",
		"eval(",
		"__",        // Potential for accessing private fields
		"reflect.", // No reflection
	}

	lowerExpr := strings.ToLower(exprStr)
	for _, pattern := range dangerous {
		if strings.Contains(lowerExpr, strings.ToLower(pattern)) {
			return fmt.Errorf("expression contains forbidden pattern: %s", pattern)
		}
	}

	return nil
}

// GetCompiledPattern returns compiled pattern for a route key
func (r *RouterConfig) GetCompiledPattern(pattern string) *CompiledPattern {
	if r.compiledPatterns == nil {
		return nil
	}
	return r.compiledPatterns[pattern]
}
