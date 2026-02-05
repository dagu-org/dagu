package eval

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
)

// expandVariables expands variable references in the input string using the provided options.
// It uses a resolver that aggregates all variable sources: explicit maps, EnvScope, and step map.
func expandVariables(ctx context.Context, value string, opts *Options) string {
	r := newResolver(ctx, opts)
	value = r.expandReferences(ctx, value)
	value = r.replaceVars(value)
	return value
}

// String substitutes environment variables and commands in the input string.
// It runs the default pipeline: quoted-refs → variables → substitute → shell-expand.
func String(ctx context.Context, input string, opts ...Option) (string, error) {
	if input == "" {
		return "", nil
	}

	options := NewOptions()
	for _, opt := range opts {
		opt(options)
	}

	return defaultPipeline.execute(ctx, input, options)
}

// IntString evaluates the input string and converts the result to an integer.
// It uses the same pipeline as String to ensure consistent evaluation ordering.
func IntString(ctx context.Context, input string, opts ...Option) (int, error) {
	value, err := String(ctx, input, opts...)
	if err != nil {
		return 0, err
	}
	v, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("failed to convert %q to int: %w", value, err)
	}
	return v, nil
}

// StringFields processes all string fields in a struct or map by expanding environment
// variables and substituting command outputs. It takes a struct or map value and returns a new
// modified struct or map value.
func StringFields[T any](ctx context.Context, obj T, opts ...Option) (T, error) {
	options := NewOptions()
	for _, opt := range opts {
		opt(options)
	}

	v := reflect.ValueOf(obj)

	// nolint:exhaustive
	switch v.Kind() {
	case reflect.Struct, reflect.Map:
		transform := func(ctx context.Context, s string) (string, error) {
			return evalStringValue(ctx, s, options)
		}
		result, err := walkValue(ctx, v, transform)
		if err != nil {
			return obj, err
		}
		return result.Interface().(T), nil

	default:
		return obj, fmt.Errorf("input must be a struct or map, got %T", obj)
	}
}

// ExpandReferences finds all occurrences of ${NAME.foo.bar} in the input string,
// where "NAME" matches a key in the dataMap. The dataMap value is expected to be
// JSON. It then uses gojq to extract the .foo.bar sub-path from that JSON
// document. If successful, it replaces the original placeholder with the sub-path value.
//
// If dataMap[name] is invalid JSON or the sub-path does not exist,
// the placeholder is left as-is.
func ExpandReferences(ctx context.Context, input string, dataMap map[string]string) string {
	return ExpandReferencesWithSteps(ctx, input, dataMap, nil)
}

// ExpandReferencesWithSteps is like ExpandReferences but also handles step ID property access
// like ${step_id.stdout}, ${step_id.stderr}, ${step_id.exit_code}
func ExpandReferencesWithSteps(ctx context.Context, input string, dataMap map[string]string, stepMap map[string]StepInfo) string {
	r := &resolver{
		variables: []map[string]string{dataMap},
		stepMap:   stepMap,
		scope:     GetEnvScope(ctx),
		expandOS:  true,
	}
	return r.expandReferences(ctx, input)
}

// Object recursively evaluates the string fields of the given object.
// It uses variable expansion, command substitution, and basic env expansion
// (via evalStringValue). OS environment is included for backward compatibility.
func Object[T any](ctx context.Context, obj T, vars map[string]string) (T, error) {
	options := NewOptions()
	options.ExpandOS = true
	WithVariables(vars)(options)

	transform := func(ctx context.Context, s string) (string, error) {
		return evalStringValue(ctx, s, options)
	}

	v := reflect.ValueOf(obj)
	result, err := walkValue(ctx, v, transform)
	if err != nil {
		return obj, err
	}

	if _, ok := result.Interface().(T); !ok {
		return obj, fmt.Errorf("expected type %T but got %T", obj, result.Interface())
	}
	return result.Interface().(T), nil
}
