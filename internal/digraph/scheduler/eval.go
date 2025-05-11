package scheduler

import (
	"context"
	"reflect"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/digraph/executor"
)

// EvalString evaluates the given string with the variables within the execution context.
func EvalString(ctx context.Context, s string, opts ...cmdutil.EvalOption) (string, error) {
	return executor.GetEnv(ctx).EvalString(ctx, s, opts...)
}

// EvalBool evaluates the given value with the variables within the execution context
// and parses it as a boolean.
func EvalBool(ctx context.Context, value any) (bool, error) {
	return executor.GetEnv(ctx).EvalBool(ctx, value)
}

// EvalObject recursively evaluates the string fields of the given object
// with the variables within the execution context.
func EvalObject[T any](ctx context.Context, obj T) (T, error) {
	env := executor.GetEnv(ctx).Variables.Variables()

	// Get the value of the object
	v := reflect.ValueOf(obj)

	// Handle different types
	switch v.Kind() {
	case reflect.Struct:
		// Use the existing cmdutil.EvalStringFields for structs
		return cmdutil.EvalStringFields(ctx, obj, cmdutil.WithVariables(env))
	case reflect.Map:
		// Process maps recursively
		result, err := processMap(ctx, v, env)
		if err != nil {
			return obj, err
		}
		return result.Interface().(T), nil
	default:
		// For other types, we can just return the object as is
		return obj, nil
	}
}

// processMap recursively processes a map, evaluating string values and recursively processing
// nested maps and structs.
func processMap(ctx context.Context, v reflect.Value, vars map[string]string) (reflect.Value, error) {
	// Create a new map of the same type
	mapType := v.Type()
	newMap := reflect.MakeMap(mapType)

	// Iterate over the map entries
	iter := v.MapRange()
	for iter.Next() {
		key := iter.Key()
		val := iter.Value()

		for (val.Kind() == reflect.Interface || val.Kind() == reflect.Ptr) && !val.IsNil() {
			val = val.Elem()
		}

		// Process the value based on its type
		var newVal reflect.Value
		var err error

		switch val.Kind() {
		case reflect.String:
			// Evaluate string values using cmdutil.EvalString
			strVal := val.String()
			evalResult, err := cmdutil.EvalString(ctx, strVal, cmdutil.WithVariables(vars))
			if err != nil {
				return v, err
			}
			newVal = reflect.ValueOf(evalResult)
		case reflect.Map:
			// Recursively process nested maps
			newVal, err = processMap(ctx, val, vars)
			if err != nil {
				return v, err
			}
		case reflect.Struct:
			// Process structs using cmdutil.EvalStringFields
			structVal := val.Interface()
			evalOpts := cmdutil.WithVariables(vars)
			processed, err := cmdutil.EvalStringFields(ctx, structVal, evalOpts)
			if err != nil {
				return v, err
			}
			newVal = reflect.ValueOf(processed)
		default:
			newVal = val
		}

		// Set the new value in the map
		newMap.SetMapIndex(key, newVal)
	}

	return newMap, nil
}
