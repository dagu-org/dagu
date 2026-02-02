package core

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
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

// EvaluationContext holds the runtime values needed for expression evaluation.
type EvaluationContext struct {
	Value    string // The evaluated router.value (e.g., stdout, evaluated expression)
	ExitCode int    // Exit code from previous step
}

// EvaluateExpression safely evaluates an expression with the given context.
// Returns true if the expression evaluates to true, false otherwise.
// This is a safe evaluator that does NOT use eval, reflection, or code execution.
//
// Supported syntax:
//   - Literals: 'string', "string", numbers
//   - Variables: @value, @exitCode
//   - Operators: ==, !=, >, <, >=, <=, &&, ||
//   - Grouping: (expression)
//
// Security: All evaluation is done via pure parsing - no code execution.
func EvaluateExpression(expr string, ctx EvaluationContext) (bool, error) {
	if err := validateExpression(expr); err != nil {
		return false, fmt.Errorf("expression validation failed: %w", err)
	}

	tokens, err := tokenizeExpression(expr)
	if err != nil {
		return false, fmt.Errorf("tokenization failed: %w", err)
	}

	parser := &exprParser{tokens: tokens, pos: 0, ctx: ctx}
	result, err := parser.parseOr()
	if err != nil {
		return false, fmt.Errorf("evaluation failed: %w", err)
	}

	if parser.pos < len(parser.tokens) {
		return false, fmt.Errorf("unexpected token after expression: %s", parser.tokens[parser.pos].value)
	}

	return result, nil
}

// token represents a lexical token in an expression
type token struct {
	typ   tokenType
	value string
}

type tokenType int

const (
	tokenEOF tokenType = iota
	tokenLParen
	tokenRParen
	tokenString
	tokenNumber
	tokenVariable // @value or @exitCode
	tokenOperator // ==, !=, >, <, >=, <=, &&, ||
)

// tokenizeExpression breaks an expression into tokens
func tokenizeExpression(expr string) ([]token, error) {
	var tokens []token
	i := 0
	expr = strings.TrimSpace(expr)

	for i < len(expr) {
		// Skip whitespace
		for i < len(expr) && (expr[i] == ' ' || expr[i] == '\t' || expr[i] == '\n') {
			i++
		}
		if i >= len(expr) {
			break
		}

		ch := expr[i]

		// Parentheses
		if ch == '(' {
			tokens = append(tokens, token{typ: tokenLParen, value: "("})
			i++
			continue
		}
		if ch == ')' {
			tokens = append(tokens, token{typ: tokenRParen, value: ")"})
			i++
			continue
		}

		// String literals (single or double quoted)
		if ch == '\'' || ch == '"' {
			quote := ch
			start := i
			i++ // skip opening quote
			for i < len(expr) && expr[i] != quote {
				if expr[i] == '\\' && i+1 < len(expr) {
					i += 2 // skip escape sequence
				} else {
					i++
				}
			}
			if i >= len(expr) {
				return nil, fmt.Errorf("unterminated string literal starting at position %d", start)
			}
			i++ // skip closing quote
			// Extract the string content without quotes
			content := expr[start+1 : i-1]
			// Unescape basic sequences
			content = strings.ReplaceAll(content, `\'`, `'`)
			content = strings.ReplaceAll(content, `\"`, `"`)
			content = strings.ReplaceAll(content, `\\`, `\`)
			tokens = append(tokens, token{typ: tokenString, value: content})
			continue
		}

		// Variables (@value, @exitCode)
		if ch == '@' {
			start := i
			i++
			for i < len(expr) && (isAlphaNumeric(expr[i]) || expr[i] == '_') {
				i++
			}
			varName := expr[start:i]
			if varName != "@value" && varName != "@exitCode" {
				return nil, fmt.Errorf("unknown variable: %s", varName)
			}
			tokens = append(tokens, token{typ: tokenVariable, value: varName})
			continue
		}

		// Numbers
		if isDigit(ch) || (ch == '-' && i+1 < len(expr) && isDigit(expr[i+1])) {
			start := i
			if ch == '-' {
				i++
			}
			for i < len(expr) && isDigit(expr[i]) {
				i++
			}
			tokens = append(tokens, token{typ: tokenNumber, value: expr[start:i]})
			continue
		}

		// Operators (check two-character operators first)
		if i+1 < len(expr) {
			twoChar := expr[i : i+2]
			if twoChar == "==" || twoChar == "!=" || twoChar == ">=" || twoChar == "<=" || twoChar == "&&" || twoChar == "||" {
				tokens = append(tokens, token{typ: tokenOperator, value: twoChar})
				i += 2
				continue
			}
		}

		// Single-character operators
		if ch == '>' || ch == '<' {
			tokens = append(tokens, token{typ: tokenOperator, value: string(ch)})
			i++
			continue
		}

		return nil, fmt.Errorf("unexpected character at position %d: %c", i, ch)
	}

	return tokens, nil
}

func isAlphaNumeric(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

// exprParser parses and evaluates expressions using recursive descent
type exprParser struct {
	tokens []token
	pos    int
	ctx    EvaluationContext
}

func (p *exprParser) current() token {
	if p.pos >= len(p.tokens) {
		return token{typ: tokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *exprParser) advance() {
	p.pos++
}

// parseOr handles || (logical OR) - lowest precedence
func (p *exprParser) parseOr() (bool, error) {
	left, err := p.parseAnd()
	if err != nil {
		return false, err
	}

	for p.current().typ == tokenOperator && p.current().value == "||" {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return false, err
		}
		left = left || right
	}

	return left, nil
}

// parseAnd handles && (logical AND)
func (p *exprParser) parseAnd() (bool, error) {
	left, err := p.parseComparison()
	if err != nil {
		return false, err
	}

	for p.current().typ == tokenOperator && p.current().value == "&&" {
		p.advance()
		right, err := p.parseComparison()
		if err != nil {
			return false, err
		}
		left = left && right
	}

	return left, nil
}

// parseComparison handles ==, !=, >, <, >=, <=
func (p *exprParser) parseComparison() (bool, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return false, err
	}

	tok := p.current()
	if tok.typ != tokenOperator {
		// No comparison operator, treat as boolean (non-empty string = true)
		return left != "", nil
	}

	op := tok.value
	if op == "&&" || op == "||" {
		// Not a comparison operator, return early
		return left != "", nil
	}

	p.advance()
	right, err := p.parsePrimary()
	if err != nil {
		return false, err
	}

	return compareValues(left, op, right)
}

// parsePrimary handles literals, variables, and grouped expressions
func (p *exprParser) parsePrimary() (string, error) {
	tok := p.current()

	switch tok.typ {
	case tokenString:
		p.advance()
		return tok.value, nil

	case tokenNumber:
		p.advance()
		return tok.value, nil

	case tokenVariable:
		p.advance()
		switch tok.value {
		case "@value":
			return p.ctx.Value, nil
		case "@exitCode":
			return fmt.Sprintf("%d", p.ctx.ExitCode), nil
		default:
			return "", fmt.Errorf("unknown variable: %s", tok.value)
		}

	case tokenLParen:
		p.advance()
		result, err := p.parseOr()
		if err != nil {
			return "", err
		}
		if p.current().typ != tokenRParen {
			return "", fmt.Errorf("expected ')', got %v", p.current())
		}
		p.advance()
		// Convert boolean result to string
		if result {
			return "true", nil
		}
		return "", nil

	default:
		return "", fmt.Errorf("unexpected token: %v", tok)
	}
}

// compareValues compares two string values using the given operator
func compareValues(left, op, right string) (bool, error) {
	// Try numeric comparison first
	leftNum, leftErr := strconv.ParseFloat(left, 64)
	rightNum, rightErr := strconv.ParseFloat(right, 64)

	if leftErr == nil && rightErr == nil {
		// Both are numbers, do numeric comparison
		switch op {
		case "==":
			return leftNum == rightNum, nil
		case "!=":
			return leftNum != rightNum, nil
		case ">":
			return leftNum > rightNum, nil
		case "<":
			return leftNum < rightNum, nil
		case ">=":
			return leftNum >= rightNum, nil
		case "<=":
			return leftNum <= rightNum, nil
		default:
			return false, fmt.Errorf("unknown operator: %s", op)
		}
	}

	// Fall back to string comparison
	switch op {
	case "==":
		return left == right, nil
	case "!=":
		return left != right, nil
	case ">":
		return left > right, nil
	case "<":
		return left < right, nil
	case ">=":
		return left >= right, nil
	case "<=":
		return left <= right, nil
	default:
		return false, fmt.Errorf("unknown operator: %s", op)
	}
}

// MatchPattern checks if a value matches a compiled pattern.
// This function is thread-safe and uses the pre-compiled pattern for performance.
//
// Pattern types:
//   - Plain string: exact match
//   - Regex: /^pattern$/ - uses pre-compiled regex (ReDoS protection applied at compile time)
//   - Array: [val1, val2, val3] - matches if value is in the array
//   - Expression: @value == 'success' - evaluates expression safely
//
// Returns true if the value matches the pattern.
func MatchPattern(compiled *CompiledPattern, value string, exitCode int) (bool, error) {
	if compiled == nil {
		return false, fmt.Errorf("compiled pattern is nil")
	}

	// Regex pattern
	if compiled.IsRegex {
		if compiled.Regex == nil {
			return false, fmt.Errorf("regex pattern not compiled")
		}
		return compiled.Regex.MatchString(value), nil
	}

	// Array pattern - check if value is in the array
	if compiled.IsArray {
		for _, v := range compiled.Values {
			if v == value {
				return true, nil
			}
		}
		return false, nil
	}

	// Expression pattern - evaluate the expression
	if compiled.IsExpression {
		ctx := EvaluationContext{
			Value:    value,
			ExitCode: exitCode,
		}
		result, err := EvaluateExpression(compiled.Original, ctx)
		if err != nil {
			return false, fmt.Errorf("expression evaluation failed: %w", err)
		}
		return result, nil
	}

	// Plain string - exact match
	return value == compiled.Original, nil
}

// EvaluateRoutes evaluates all route patterns and returns the steps to activate.
// This implements both exclusive and multi-select modes with proper security.
//
// Mode behavior:
//   - exclusive: Returns steps from the first matching pattern only
//   - multi-select: Returns steps from all matching patterns (deduplicated)
//
// If no patterns match, returns the default steps.
// Thread-safe: uses pre-compiled patterns from RouterConfig.Validate().
func (r *RouterConfig) EvaluateRoutes(value string, exitCode int) ([]string, []string, error) {
	if r.compiledPatterns == nil {
		return nil, nil, fmt.Errorf("router patterns not compiled (call Validate first)")
	}

	var matchedPatterns []string
	var activatedSteps []string
	seenSteps := make(map[string]bool)

	// Iterate through routes in deterministic order (map iteration in Go 1.12+ is randomized,
	// but we store patterns during Validate, so we iterate through the routes map)
	// For exclusive mode, we need to ensure consistent ordering
	patterns := make([]string, 0, len(r.Routes))
	for pattern := range r.Routes {
		patterns = append(patterns, pattern)
	}
	// Sort patterns for deterministic evaluation order
	// This ensures exclusive mode always picks the same "first" match
	sortPatterns(patterns)

	for _, pattern := range patterns {
		compiled := r.compiledPatterns[pattern]
		if compiled == nil {
			// Should never happen if Validate was called
			continue
		}

		matched, err := MatchPattern(compiled, value, exitCode)
		if err != nil {
			// Log error but continue to other patterns (defense in depth)
			continue
		}

		if matched {
			matchedPatterns = append(matchedPatterns, pattern)
			steps := r.Routes[pattern]

			// Add steps (with deduplication)
			for _, step := range steps {
				if !seenSteps[step] {
					seenSteps[step] = true
					activatedSteps = append(activatedSteps, step)
				}
			}

			// Exclusive mode: stop after first match
			if r.Mode == RouterModeExclusive {
				break
			}
		}
	}

	// If no patterns matched, use default steps
	if len(matchedPatterns) == 0 {
		for _, step := range r.Default {
			if !seenSteps[step] {
				seenSteps[step] = true
				activatedSteps = append(activatedSteps, step)
			}
		}
	}

	return matchedPatterns, activatedSteps, nil
}

// sortPatterns sorts patterns to ensure deterministic evaluation order.
// Sorting strategy:
//   1. Regex patterns first (most specific)
//   2. Array patterns second
//   3. Expression patterns third
//   4. Plain strings last (least specific)
//   5. Within each group, alphabetical order
func sortPatterns(patterns []string) {
	// Simple sort - more sophisticated prioritization could be added
	// For now, we use lexicographic order which is deterministic
	for i := 0; i < len(patterns); i++ {
		for j := i + 1; j < len(patterns); j++ {
			// Regex patterns (start with /) should come first
			iIsRegex := strings.HasPrefix(patterns[i], "/")
			jIsRegex := strings.HasPrefix(patterns[j], "/")
			if jIsRegex && !iIsRegex {
				patterns[i], patterns[j] = patterns[j], patterns[i]
				continue
			}
			if iIsRegex && !jIsRegex {
				continue
			}

			// Array patterns (start with [) should come second
			iIsArray := strings.HasPrefix(patterns[i], "[")
			jIsArray := strings.HasPrefix(patterns[j], "[")
			if jIsArray && !iIsArray {
				patterns[i], patterns[j] = patterns[j], patterns[i]
				continue
			}
			if iIsArray && !jIsArray {
				continue
			}

			// Within same type, use alphabetical order
			if patterns[i] > patterns[j] {
				patterns[i], patterns[j] = patterns[j], patterns[i]
			}
		}
	}
}
