package cmdutil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/itchyny/gojq"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/syntax"
)

type EvalOptions struct {
	ExpandEnv   bool
	Substitute  bool
	Variables   []map[string]string
	StepMap     map[string]StepInfo
	ExpandShell bool // When false, skip shell-based variable expansion (e.g., for SSH commands)
}

func NewEvalOptions() *EvalOptions {
	return &EvalOptions{
		ExpandEnv:   true,
		ExpandShell: true,
		Substitute:  true,
	}
}

type EvalOption func(*EvalOptions)

func WithVariables(vars map[string]string) EvalOption {
	return func(opts *EvalOptions) {
		opts.Variables = append(opts.Variables, vars)
	}
}

func WithStepMap(stepMap map[string]StepInfo) EvalOption {
	return func(opts *EvalOptions) {
		opts.StepMap = stepMap
	}
}

func WithoutExpandEnv() EvalOption {
	return func(opts *EvalOptions) {
		opts.ExpandEnv = false
	}
}

func WithoutExpandShell() EvalOption {
	return func(opts *EvalOptions) {
		opts.ExpandShell = false
	}
}

func WithoutSubstitute() EvalOption {
	return func(opts *EvalOptions) {
		opts.Substitute = false
	}
}

func OnlyReplaceVars() EvalOption {
	return func(opts *EvalOptions) {
		opts.ExpandEnv = false
		opts.Substitute = false
	}
}

var reEscapedKeyValue = regexp.MustCompile(`^[^\s=]+="[^"]+"$`)

// reVarSubstitution matches $VAR, ${VAR}, '$VAR', '${VAR}' patterns for variable substitution.
// Group 1: ${...} content, Group 2: $VAR content (without braces)
var reVarSubstitution = regexp.MustCompile(`[']{0,1}\$\{([^}]+)\}[']{0,1}|[']{0,1}\$([a-zA-Z0-9_][a-zA-Z0-9_]*)[']{0,1}`)

// reQuotedJSONRef matches quoted JSON references like "${FOO.bar}" and simple variables like "${VAR}"
var reQuotedJSONRef = regexp.MustCompile(`"\$\{([A-Za-z0-9_]\w*(?:\.[^}]+)?)\}"`)

// reJSONPathRef matches patterns like ${FOO.bar.baz} or $FOO.bar for JSON path expansion
var reJSONPathRef = regexp.MustCompile(`\$\{([A-Za-z0-9_]\w*)(\.[^}]+)\}|\$([A-Za-z0-9_]\w*)(\.[^\s]+)`)

// BuildCommandEscapedString constructs a single shell-ready string from a command and its arguments.
// It assumes that the command and arguments are already escaped.
func BuildCommandEscapedString(command string, args []string) string {
	quotedArgs := make([]string, 0, len(args))
	for _, arg := range args {
		// If already quoted, skip
		if strings.HasPrefix(arg, `"`) && strings.HasSuffix(arg, `"`) {
			quotedArgs = append(quotedArgs, arg)
			continue
		}
		if strings.HasPrefix(arg, `'`) && strings.HasSuffix(arg, `'`) {
			quotedArgs = append(quotedArgs, arg)
			continue
		}
		// If the argument contains spaces, quote it.
		if strings.ContainsAny(arg, " ") {
			// If it includes '=' and is already quoted, skip
			if reEscapedKeyValue.MatchString(arg) {
				quotedArgs = append(quotedArgs, arg)
				continue
			}
			// if it contains double quotes, escape them
			arg = strings.ReplaceAll(arg, `"`, `\"`)
			quotedArgs = append(quotedArgs, fmt.Sprintf(`"%s"`, arg))
		} else {
			quotedArgs = append(quotedArgs, arg)
		}
	}

	// If we have no arguments, just return the command without trailing space.
	if len(quotedArgs) == 0 {
		return command
	}

	return fmt.Sprintf("%s %s", command, strings.Join(quotedArgs, " "))
}

// expandVariables expands variable references in the input string using the provided options.
// It uses opts.Variables for explicit variable maps, and falls back to EnvScope from context.
func expandVariables(ctx context.Context, value string, options *EvalOptions) string {
	// Handle step references like ${step.stdout} first
	if options.StepMap != nil {
		value = ExpandReferencesWithSteps(ctx, value, map[string]string{}, options.StepMap)
	}

	// Expand from explicit Variables maps
	for _, vars := range options.Variables {
		if options.StepMap != nil {
			value = ExpandReferencesWithSteps(ctx, value, vars, options.StepMap)
		} else {
			value = ExpandReferences(ctx, value, vars)
		}
		value = replaceVars(value, vars)
	}

	// Also expand from EnvScope if available (for params like $1, $2)
	// This ensures variables in EnvScope are expanded even when opts.Variables is empty
	if scope := GetEnvScope(ctx); scope != nil {
		value = replaceVarsWithScope(value, scope)
	}

	return value
}

// EvalString substitutes environment variables and commands in the input string
func EvalString(ctx context.Context, input string, opts ...EvalOption) (string, error) {
	if input == "" {
		return "", nil // nothing to do
	}

	options := NewEvalOptions()
	for _, opt := range opts {
		opt(options)
	}
	value := input

	// Expand quoted values first (including JSON paths)
	for _, vars := range options.Variables {
		// Handle quoted JSON references like "${FOO.bar}" and simple variables like "${VAR}"
		value = reQuotedJSONRef.ReplaceAllStringFunc(value, func(match string) string {
			// Extract the reference (VAR or VAR.path)
			ref := match[3 : len(match)-2] // Remove "$ and }"

			// Check if it's a JSON path reference
			if strings.Contains(ref, ".") {
				// JSON path - extract using existing logic
				testRef := "${" + ref + "}"
				var extracted string
				if options.StepMap != nil {
					extracted = ExpandReferencesWithSteps(ctx, testRef, vars, options.StepMap)
				} else {
					extracted = ExpandReferences(ctx, testRef, vars)
				}
				if extracted != testRef { // Successfully extracted
					// strconv.Quote already includes the outer quotes
					return strconv.Quote(extracted)
				}
			} else {
				// Simple variable
				if val, ok := vars[ref]; ok {
					// strconv.Quote already includes the outer quotes
					return strconv.Quote(val)
				}
			}
			return match // Keep original if not found
		})
	}

	value = expandVariables(ctx, value, options)

	if options.Substitute {
		var err error
		value, err = substituteCommandsWithContext(ctx, value)
		if err != nil {
			return "", fmt.Errorf("failed to substitute string in %q: %w", input, err)
		}
	}
	if options.ExpandEnv {
		expanded, err := expandWithShellContext(ctx, value, options)
		if err != nil {
			logger.Debug(ctx, "Shell expansion failed, falling back to ExpandEnvContext",
				tag.Error(err))
			value = ExpandEnvContext(ctx, value)
		} else {
			value = expanded
		}
	}
	return value, nil
}

// EvalIntString substitutes environment variables and commands in the input string
func EvalIntString(ctx context.Context, input string, opts ...EvalOption) (int, error) {
	options := NewEvalOptions()
	for _, opt := range opts {
		opt(options)
	}
	value := input

	value = expandVariables(ctx, value, options)

	if options.ExpandEnv {
		expanded, err := expandWithShellContext(ctx, value, options)
		if err != nil {
			logger.Debug(ctx, "Shell expansion failed, falling back to ExpandEnvContext",
				tag.Error(err))
			value = ExpandEnvContext(ctx, value)
		} else {
			value = expanded
		}
	}

	if options.Substitute {
		var err error
		value, err = substituteCommandsWithContext(ctx, value)
		if err != nil {
			return 0, err
		}
	}
	v, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("failed to convert %q to int: %w", value, err)
	}
	return v, nil
}

// EvalStringFields processes all string fields in a struct or map by expanding environment
// variables and substituting command outputs. It takes a struct or map value and returns a new
// modified struct or map value.
func EvalStringFields[T any](ctx context.Context, obj T, opts ...EvalOption) (T, error) {
	options := NewEvalOptions()
	for _, opt := range opts {
		opt(options)
	}

	v := reflect.ValueOf(obj)

	// Handle different types
	// nolint:exhaustive
	switch v.Kind() {
	case reflect.Struct:
		modified := reflect.New(v.Type()).Elem()
		modified.Set(v)

		if err := processStructFields(ctx, modified, options); err != nil {
			return obj, fmt.Errorf("failed to process struct fields: %w", err)
		}

		return modified.Interface().(T), nil

	case reflect.Map:
		result, err := processMapWithOpts(ctx, v, options)
		if err != nil {
			return obj, fmt.Errorf("failed to process map: %w", err)
		}
		return result.Interface().(T), nil

	default:
		return obj, fmt.Errorf("input must be a struct or map, got %T", obj)
	}
}

// evalStringValue applies variable expansion, substitution, and env expansion to a string.
func evalStringValue(ctx context.Context, value string, opts *EvalOptions) (string, error) {
	value = expandVariables(ctx, value, opts)
	if opts.Substitute {
		var err error
		value, err = substituteCommandsWithContext(ctx, value)
		if err != nil {
			return "", err
		}
	}
	if opts.ExpandEnv {
		value = ExpandEnvContext(ctx, value)
	}
	return value, nil
}

func processStructFields(ctx context.Context, v reflect.Value, opts *EvalOptions) error {
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if !field.CanSet() {
			continue
		}

		// nolint:exhaustive
		switch field.Kind() {
		case reflect.Ptr:
			if field.IsNil() {
				continue
			}
			if err := processPointerField(ctx, field, opts); err != nil {
				return err
			}

		case reflect.String:
			value, err := evalStringValue(ctx, field.String(), opts)
			if err != nil {
				return fmt.Errorf("field %q: %w", t.Field(i).Name, err)
			}
			field.SetString(value)

		case reflect.Struct:
			if err := processStructFields(ctx, field, opts); err != nil {
				return err
			}

		case reflect.Map:
			if field.IsNil() {
				continue
			}
			processed, err := processMapWithOpts(ctx, field, opts)
			if err != nil {
				return fmt.Errorf("field %q: %w", t.Field(i).Name, err)
			}
			field.Set(processed)

		case reflect.Slice, reflect.Array:
			if field.IsNil() {
				continue
			}
			newSlice := reflect.MakeSlice(field.Type(), field.Len(), field.Cap())
			reflect.Copy(newSlice, field)
			if err := processSliceWithOpts(ctx, newSlice, opts); err != nil {
				return err
			}
			field.Set(newSlice)
		}
	}
	return nil
}

func processPointerField(ctx context.Context, field reflect.Value, opts *EvalOptions) error {
	elem := field.Elem()
	if !elem.CanSet() {
		return nil
	}

	// nolint:exhaustive
	switch elem.Kind() {
	case reflect.String:
		value, err := evalStringValue(ctx, elem.String(), opts)
		if err != nil {
			return err
		}
		newStr := reflect.New(elem.Type())
		newStr.Elem().SetString(value)
		field.Set(newStr)

	case reflect.Struct:
		newStruct := reflect.New(elem.Type())
		newStruct.Elem().Set(elem)
		if err := processStructFields(ctx, newStruct.Elem(), opts); err != nil {
			return err
		}
		field.Set(newStruct)

	case reflect.Map:
		if elem.IsNil() {
			return nil
		}
		processed, err := processMapWithOpts(ctx, elem, opts)
		if err != nil {
			return err
		}
		newMap := reflect.New(elem.Type())
		newMap.Elem().Set(processed)
		field.Set(newMap)

	case reflect.Slice, reflect.Array:
		if elem.IsNil() {
			return nil
		}
		newSlice := reflect.MakeSlice(elem.Type(), elem.Len(), elem.Cap())
		reflect.Copy(newSlice, elem)
		if err := processSliceWithOpts(ctx, newSlice, opts); err != nil {
			return err
		}
		newPtr := reflect.New(elem.Type())
		newPtr.Elem().Set(newSlice)
		field.Set(newPtr)
	}

	return nil
}

func processSliceWithOpts(ctx context.Context, v reflect.Value, opts *EvalOptions) error {
	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		if !elem.CanSet() {
			continue
		}

		// nolint:exhaustive
		switch elem.Kind() {
		case reflect.String:
			value, err := evalStringValue(ctx, elem.String(), opts)
			if err != nil {
				return err
			}
			elem.SetString(value)

		case reflect.Struct:
			if err := processStructFields(ctx, elem, opts); err != nil {
				return err
			}

		case reflect.Map:
			if elem.IsNil() {
				continue
			}
			processed, err := processMapWithOpts(ctx, elem, opts)
			if err != nil {
				return err
			}
			elem.Set(processed)

		case reflect.Ptr:
			if elem.IsNil() {
				continue
			}
			if err := processPointerField(ctx, elem, opts); err != nil {
				return err
			}
		}
	}
	return nil
}

// processMapWithOpts recursively processes a map, evaluating string values and recursively processing
// nested maps and structs.
func processMapWithOpts(ctx context.Context, v reflect.Value, opts *EvalOptions) (reflect.Value, error) {
	mapType := v.Type()
	newMap := reflect.MakeMap(mapType)

	iter := v.MapRange()
	for iter.Next() {
		key := iter.Key()
		val := iter.Value()

		for (val.Kind() == reflect.Interface || val.Kind() == reflect.Ptr) && !val.IsNil() {
			val = val.Elem()
		}

		var newVal reflect.Value
		var err error

		// nolint:exhaustive
		switch val.Kind() {
		case reflect.String:
			strVal, err := evalStringValue(ctx, val.String(), opts)
			if err != nil {
				return v, fmt.Errorf("map value: %w", err)
			}
			newVal = reflect.ValueOf(strVal)

		case reflect.Map:
			newVal, err = processMapWithOpts(ctx, val, opts)
			if err != nil {
				return v, err
			}

		case reflect.Struct:
			structCopy := reflect.New(val.Type()).Elem()
			structCopy.Set(val)
			if err := processStructFields(ctx, structCopy, opts); err != nil {
				return v, err
			}
			newVal = structCopy

		default:
			newVal = val
		}

		newMap.SetMapIndex(key, newVal)
	}

	return newMap, nil
}

// StepInfo contains metadata about a step that can be accessed via property syntax
type StepInfo struct {
	Stdout   string
	Stderr   string
	ExitCode string
}

// ExpandReferences finds all occurrences of ${NAME.foo.bar} in the input string,
// where "NAME" matches a key in the dataMap. The dataMap value is expected to be
// JSON. It then uses gojq to extract the .foo.bar sub-path from that JSON
// document. If successful, it replaces the original placeholder with the sub-path value.
//
// If dataMap[name] is invalid JSON or the sub-path does not exist,
// the placeholder is left as-is (or you could handle it differently).
func ExpandReferences(ctx context.Context, input string, dataMap map[string]string) string {
	return ExpandReferencesWithSteps(ctx, input, dataMap, nil)
}

// resolveStepProperty extracts a step's property value with optional slicing
func resolveStepProperty(ctx context.Context, stepName, path string, stepMap map[string]StepInfo) (string, bool) {
	stepInfo, ok := stepMap[stepName]
	if !ok {
		logger.Debug(ctx, "Step not found in stepMap", tag.Step(stepName))
		return "", false
	}

	property, sliceSpec, err := parseStepReference(path)
	if err != nil {
		logger.Warn(ctx, "Invalid step reference slice",
			tag.Step(stepName),
			tag.Path(path),
			tag.Error(err))
		return "", false
	}

	var value string
	switch property {
	case ".stdout":
		if stepInfo.Stdout == "" {
			logger.Debug(ctx, "Step stdout is empty",
				tag.Step(stepName))
			return "", false
		}
		value = stepInfo.Stdout
	case ".stderr":
		if stepInfo.Stderr == "" {
			logger.Debug(ctx, "Step stderr is empty",
				tag.Step(stepName))
			return "", false
		}
		value = stepInfo.Stderr
	case ".exitCode", ".exit_code":
		value = stepInfo.ExitCode
	default:
		return "", false
	}

	if sliceSpec.hasStart || sliceSpec.hasLength {
		value = applyStepSlice(value, sliceSpec)
	}

	return value, true
}

// resolveJSONPath extracts a value from JSON data using a jq-style path
func resolveJSONPath(ctx context.Context, varName, jsonStr, path string) (string, bool) {
	var raw any
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		logger.Warn(ctx, "Failed to parse JSON",
			slog.String("var", varName),
			tag.Error(err))
		return "", false
	}

	query, err := gojq.Parse(path)
	if err != nil {
		logger.Warn(ctx, "Failed to parse path in data",
			tag.Path(path),
			slog.String("var", varName),
			tag.Error(err))
		return "", false
	}

	iter := query.Run(raw)
	v, ok := iter.Next()
	if !ok {
		return "", false
	}

	if _, isErr := v.(error); isErr {
		logger.Warn(ctx, "Error evaluating path in data",
			tag.Path(path),
			slog.String("var", varName),
			tag.Error(v))
		return "", false
	}

	return fmt.Sprintf("%v", v), true
}

// ExpandReferencesWithSteps is like ExpandReferences but also handles step ID property access
// like ${step_id.stdout}, ${step_id.stderr}, ${step_id.exit_code}
func ExpandReferencesWithSteps(ctx context.Context, input string, dataMap map[string]string, stepMap map[string]StepInfo) string {
	return reJSONPathRef.ReplaceAllStringFunc(input, func(match string) string {
		subMatches := reJSONPathRef.FindStringSubmatch(match)
		if len(subMatches) < 3 {
			return match
		}

		var varName, path string
		if strings.HasPrefix(subMatches[0], "${") {
			varName = subMatches[1]
			path = subMatches[2]
		} else {
			varName = subMatches[3]
			path = subMatches[4]
		}

		// Try step property resolution first
		if stepMap != nil {
			if value, ok := resolveStepProperty(ctx, varName, path, stepMap); ok {
				return value
			}
		}

		// Try regular variable or environment lookup
		jsonStr, ok := dataMap[varName]
		if !ok {
			// Try EnvScope from context first, then fall back to os.LookupEnv
			if scope := GetEnvScope(ctx); scope != nil {
				if envVal, exists := scope.Get(varName); exists {
					jsonStr = envVal
				} else {
					return match
				}
			} else if envVal, exists := os.LookupEnv(varName); exists {
				jsonStr = envVal
			} else {
				return match
			}
		}

		// Try JSON path resolution
		if value, ok := resolveJSONPath(ctx, varName, jsonStr, path); ok {
			return value
		}

		return match
	})
}

// extractVarKey extracts the variable key from a regex match.
// Returns the key and false if the match should be skipped (single-quoted).
func extractVarKey(match string) (string, bool) {
	if match[0] == '\'' && match[len(match)-1] == '\'' {
		return "", false // Single-quoted - skip
	}
	if strings.HasPrefix(match, "${") {
		return match[2 : len(match)-1], true
	}
	return match[1:], true
}

// replaceVars substitutes $VAR and ${VAR} patterns using the provided map.
func replaceVars(template string, vars map[string]string) string {
	return reVarSubstitution.ReplaceAllStringFunc(template, func(match string) string {
		key, ok := extractVarKey(match)
		if !ok {
			return match
		}
		if val, found := vars[key]; found {
			return val
		}
		return match
	})
}

// replaceVarsWithScope substitutes $VAR and ${VAR} patterns using EnvScope.
//
// This function intentionally skips OS-sourced variables (EnvSourceOS) during
// early expansion. The rationale is:
//
//  1. OS environment variables may change between DAG load time and step execution
//     (e.g., PATH modifications, dynamic tokens).
//  2. By deferring OS var expansion to shell execution time (via expandWithShellContext),
//     we ensure the shell reads the current OS environment when the command runs.
//  3. User-defined variables (DAG env, step env, secrets, outputs) are stable and
//     can be safely expanded early.
//
// This design allows commands like "echo $PATH" to use the live OS PATH at execution
// time, rather than a stale value captured when the DAG was loaded.
func replaceVarsWithScope(template string, scope *EnvScope) string {
	return reVarSubstitution.ReplaceAllStringFunc(template, func(match string) string {
		key, ok := extractVarKey(match)
		if !ok {
			return match
		}
		// Skip JSON paths (handled elsewhere)
		if strings.Contains(key, ".") {
			return match
		}
		// Only expand user-defined vars, not OS-sourced (see function doc for rationale)
		if entry, found := scope.GetEntry(key); found && entry.Source != EnvSourceOS {
			return entry.Value
		}
		return match
	})
}

func expandWithShellContext(ctx context.Context, input string, opts *EvalOptions) (string, error) {
	if !opts.ExpandShell {
		if !opts.ExpandEnv {
			return input, nil
		}
		return ExpandEnvContext(ctx, input), nil
	}

	parser := syntax.NewParser()
	word, err := parser.Document(strings.NewReader(input))
	if err != nil {
		return "", err
	}
	if word == nil {
		return "", nil
	}

	cfg := &expand.Config{
		Env: expand.FuncEnviron(func(name string) string {
			if val, ok := lookupVariable(name, opts.Variables); ok {
				return val
			}
			// Check EnvScope from context first, then fall back to os.Getenv
			if scope := GetEnvScope(ctx); scope != nil {
				// Only use USER-defined vars from scope, not OS-sourced vars
				// This lets us read live OS env values instead of frozen ones
				if entry, ok := scope.GetEntry(name); ok && entry.Source != EnvSourceOS {
					return entry.Value
				}
			}
			return os.Getenv(name)
		}),
	}

	result, err := expand.Literal(cfg, word)
	if err != nil {
		var unexpected expand.UnexpectedCommandError
		if errors.As(err, &unexpected) {
			return ExpandEnvContext(ctx, input), nil
		}
		return "", err
	}
	return result, nil
}

func lookupVariable(name string, scopes []map[string]string) (string, bool) {
	for _, vars := range scopes {
		if val, ok := vars[name]; ok {
			return val, true
		}
	}
	return "", false
}

type stepSliceSpec struct {
	hasStart  bool
	start     int
	hasLength bool
	length    int
}

// parseStepReference parses a step reference path like ".stdout:0:5" into property and slice spec
// Returns the property name (e.g., ".stdout") and slice specification (start, length)
func parseStepReference(path string) (string, stepSliceSpec, error) {
	spec := stepSliceSpec{}

	colonIdx := strings.Index(path, ":")
	if colonIdx == -1 {
		return path, spec, nil
	}

	property := path[:colonIdx]
	sliceNotation := path[colonIdx+1:]

	if sliceNotation == "" {
		return "", spec, fmt.Errorf("slice specification missing values")
	}

	parts := strings.Split(sliceNotation, ":")
	if len(parts) > 2 {
		return "", spec, fmt.Errorf("too many slice sections")
	}

	// Parse start offset (required)
	if parts[0] == "" {
		return "", spec, fmt.Errorf("slice offset is required")
	}

	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", spec, fmt.Errorf("invalid slice offset: %w", err)
	}
	if start < 0 {
		return "", spec, fmt.Errorf("slice offset must be non-negative")
	}
	spec.hasStart = true
	spec.start = start

	// Parse length (optional)
	if len(parts) == 2 && parts[1] != "" {
		length, err := strconv.Atoi(parts[1])
		if err != nil {
			return "", spec, fmt.Errorf("invalid slice length: %w", err)
		}
		if length < 0 {
			return "", spec, fmt.Errorf("slice length must be non-negative")
		}
		spec.hasLength = true
		spec.length = length
	}

	return property, spec, nil
}

// applyStepSlice applies substring slicing to a string value based on the slice specification
// Similar to Python/shell string slicing: value[start:start+length]
func applyStepSlice(value string, spec stepSliceSpec) string {
	if !spec.hasStart {
		return value
	}

	runes := []rune(value)
	if spec.start >= len(runes) {
		return ""
	}

	endIdx := len(runes)
	if spec.hasLength {
		endIdx = spec.start + spec.length
		if endIdx > len(runes) {
			endIdx = len(runes)
		}
	}

	return string(runes[spec.start:endIdx])
}
