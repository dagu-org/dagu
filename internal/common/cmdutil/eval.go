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

	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/itchyny/gojq"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/syntax"
)

type EvalOptions struct {
	ExpandEnv  bool
	Substitute bool
	Variables  []map[string]string
	StepMap    map[string]StepInfo
}

func NewEvalOptions() *EvalOptions {
	return &EvalOptions{
		ExpandEnv:  true,
		Substitute: true,
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

// expandVariables expands variable references in the input string using the provided options
func expandVariables(ctx context.Context, value string, options *EvalOptions) string {
	if len(options.Variables) == 0 && options.StepMap != nil {
		return ExpandReferencesWithSteps(ctx, value, map[string]string{}, options.StepMap)
	}

	for _, vars := range options.Variables {
		if options.StepMap != nil {
			value = ExpandReferencesWithSteps(ctx, value, vars, options.StepMap)
		} else {
			value = ExpandReferences(ctx, value, vars)
		}
		value = replaceVars(value, vars)
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
		quotedRefPattern := regexp.MustCompile(`"\$\{([A-Za-z0-9_]\w*(?:\.[^}]+)?)\}"`)
		value = quotedRefPattern.ReplaceAllStringFunc(value, func(match string) string {
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
		value, err = substituteCommands(value)
		if err != nil {
			return "", fmt.Errorf("failed to substitute string in %q: %w", input, err)
		}
	}
	if options.ExpandEnv {
		expanded, err := expandWithShell(value, options)
		if err != nil {
			logger.Debug(ctx, "Shell expansion failed, falling back to os.ExpandEnv",
				tag.Error(err))
			value = os.ExpandEnv(value)
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
		expanded, err := expandWithShell(value, options)
		if err != nil {
			logger.Debug(ctx, "Shell expansion failed, falling back to os.ExpandEnv",
				tag.Error(err))
			value = os.ExpandEnv(value)
		} else {
			value = expanded
		}
	}
	value, err := substituteCommands(value)
	if err != nil {
		return 0, err
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

func processStructFields(ctx context.Context, v reflect.Value, opts *EvalOptions) error {
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if !field.CanSet() {
			continue
		}

		// nolint:exhaustive
		switch field.Kind() {
		case reflect.String:
			value := field.String()

			value = expandVariables(ctx, value, opts)

			if opts.Substitute {
				var err error
				value, err = substituteCommands(value)
				if err != nil {
					return fmt.Errorf("field %q: %w", t.Field(i).Name, err)
				}
			}

			if opts.ExpandEnv {
				value = os.ExpandEnv(value)
			}

			field.SetString(value)

		case reflect.Struct:
			if err := processStructFields(ctx, field, opts); err != nil {
				return err
			}

		case reflect.Map:
			// Process map fields
			if field.IsNil() {
				continue
			}

			processed, err := processMapWithOpts(ctx, field, opts)
			if err != nil {
				return fmt.Errorf("field %q: %w", t.Field(i).Name, err)
			}

			field.Set(processed)
		}
	}
	return nil
}

// processMapWithOpts recursively processes a map, evaluating string values and recursively processing
// nested maps and structs.
func processMapWithOpts(ctx context.Context, v reflect.Value, opts *EvalOptions) (reflect.Value, error) {
	// Create a new map of the same type
	mapType := v.Type()
	newMap := reflect.MakeMap(mapType)

	// Iterate over the map entries
	iter := v.MapRange()
	for iter.Next() {
		key := iter.Key()
		val := iter.Value()

		// Process the value based on its type
		var newVal reflect.Value
		var err error

		for (val.Kind() == reflect.Interface || val.Kind() == reflect.Ptr) && !val.IsNil() {
			val = val.Elem()
		}

		// nolint:exhaustive
		switch val.Kind() {
		case reflect.String:
			// Evaluate string values
			strVal := val.String()

			strVal = expandVariables(ctx, strVal, opts)

			if opts.Substitute {
				var err error
				strVal, err = substituteCommands(strVal)
				if err != nil {
					return v, fmt.Errorf("map value: %w", err)
				}
			}

			if opts.ExpandEnv {
				strVal = os.ExpandEnv(strVal)
			}

			newVal = reflect.ValueOf(strVal)

		case reflect.Map:
			// Recursively process nested maps
			newVal, err = processMapWithOpts(ctx, val, opts)
			if err != nil {
				return v, err
			}

		case reflect.Struct:
			// Process structs
			structCopy := reflect.New(val.Type()).Elem()
			structCopy.Set(val)

			if err := processStructFields(ctx, structCopy, opts); err != nil {
				return v, err
			}

			newVal = structCopy

		default:
			// Keep other types as is
			newVal = val
		}

		// Set the new value in the map
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
	// Regex to match patterns like ${FOO.bar.baz} or $FOO.bar
	re := regexp.MustCompile(`\$\{([A-Za-z0-9_]\w*)(\.[^}]+)\}|\$([A-Za-z0-9_]\w*)(\.[^\s]+)`)

	return re.ReplaceAllStringFunc(input, func(match string) string {
		subMatches := re.FindStringSubmatch(match)
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
			if envVal, exists := os.LookupEnv(varName); exists {
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

func replaceVars(template string, vars map[string]string) string {
	re := regexp.MustCompile(`[']{0,1}\$\{([^}]+)\}[']{0,1}|[']{0,1}\$([a-zA-Z0-9_][a-zA-Z0-9_]*)[']{0,1}`)

	return re.ReplaceAllStringFunc(template, func(match string) string {
		var key string
		if match[0] == '\'' && match[len(match)-1] == '\'' {
			// If the match is surrounded by single quotes, leave it as-is
			return match
		}
		if strings.HasPrefix(match, "${") {
			key = match[2 : len(match)-1]
		} else {
			key = match[1:]
		}

		if val, ok := vars[key]; ok {
			return val
		}
		return match
	})
}

func expandWithShell(input string, opts *EvalOptions) (string, error) {
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
			return os.Getenv(name)
		}),
	}

	result, err := expand.Literal(cfg, word)
	if err != nil {
		var unexpected expand.UnexpectedCommandError
		if errors.As(err, &unexpected) {
			return os.ExpandEnv(input), nil
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
