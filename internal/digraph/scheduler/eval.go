package scheduler

import (
	"context"
	"crypto/rand"
	"fmt"
	"reflect"
	"time"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/digraph/executor"
	"github.com/dagu-org/dagu/internal/stringutil"
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
	vars := executor.GetEnv(ctx).VariablesMap()

	result, err := eval(ctx, obj, vars)
	if err != nil {
		return obj, err
	}
	// If the result is not of type T, we return the original object
	if _, ok := result.(T); !ok {
		return obj, fmt.Errorf("expected type %T but got %T", obj, result)
	}
	// If the result is of type T, we return it
	return result.(T), nil
}

// eval evaluates the given object based on its type.
func eval(ctx context.Context, obj any, vars map[string]string) (any, error) {
	v := reflect.ValueOf(obj)
	// Handle different types
	// nolint:exhaustive
	switch v.Kind() {
	case reflect.String:
		// Evaluate string values using cmdutil.EvalString
		strVal := v.String()
		evalResult, err := cmdutil.EvalString(ctx, strVal, cmdutil.WithVariables(vars))
		if err != nil {
			return nil, err
		}
		return evalResult, nil
	case reflect.Struct:
		// Use the existing cmdutil.EvalStringFields for structs
		return cmdutil.EvalStringFields(ctx, obj, cmdutil.WithVariables(vars))
	case reflect.Map:
		// Process maps recursively
		result, err := processMap(ctx, v, vars)
		if err != nil {
			return nil, err
		}
		return result.Interface(), nil
	default:
		// For other types, we can just return the object as is
		return obj, nil
	}
}

// GenerateChildDAGRunID generates a unique run ID based on the current DAG run ID, step name, and parameters.
func GenerateChildDAGRunID(ctx context.Context, params string, repeated bool) string {
	if repeated {
		// If this is a repeated child DAG run, we need to generate a unique ID with randomness
		// to avoid collisions with previous runs.
		randomBytes := make([]byte, 8)
		if _, err := rand.Read(randomBytes); err != nil {
			randomBytes = []byte(fmt.Sprintf("%d", time.Now().UnixNano()))
		}
		return stringutil.Base58EncodeSHA256(
			fmt.Sprintf("%s:%s:%s:%x", executor.GetEnv(ctx).DAGRunID, executor.GetEnv(ctx).Step.Name, params, randomBytes),
		)
	}
	env := executor.GetEnv(ctx)
	return stringutil.Base58EncodeSHA256(
		fmt.Sprintf("%s:%s:%s", env.DAGRunID, env.Step.Name, params),
	)
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

		// nolint:exhaustive
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
		case reflect.Slice, reflect.Array:
			// Process slices and arrays by evaluating each element
			newSlice := reflect.MakeSlice(val.Type(), val.Len(), val.Cap())
			for i := 0; i < val.Len(); i++ {
				newVal, err := eval(ctx, val.Index(i).Interface(), vars)
				if err != nil {
					return v, err
				}
				// Set the evaluated value in the new slice
				newSlice.Index(i).Set(reflect.ValueOf(newVal))
			}
			// Set the new slice in the map
			newVal = newSlice
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
