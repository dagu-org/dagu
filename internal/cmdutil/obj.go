package cmdutil

import (
	"context"
	"fmt"
	"reflect"
)

// EvalObject recursively evaluates the string fields of the given object
func EvalObject[T any](ctx context.Context, obj T, vars map[string]string) (T, error) {
	result, err := evalObject(ctx, obj, vars)
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

// evalObject evaluates the given object based on its type.
func evalObject(ctx context.Context, obj any, vars map[string]string) (any, error) {
	v := reflect.ValueOf(obj)
	// Handle different types
	// nolint:exhaustive
	switch v.Kind() {
	case reflect.String:
		// Evaluate string values using cmdutil.EvalString
		strVal := v.String()
		evalResult, err := EvalString(ctx, strVal, WithVariables(vars))
		if err != nil {
			return nil, err
		}
		return evalResult, nil
	case reflect.Struct:
		// Use the existing cmdutil.EvalStringFields for structs
		return EvalStringFields(ctx, obj, WithVariables(vars))
	case reflect.Map:
		// Process maps recursively
		result, err := processMapWithVars(ctx, v, vars)
		if err != nil {
			return nil, err
		}
		return result.Interface(), nil
	case reflect.Slice, reflect.Array:
		// Process slices and arrays recursively
		newSlice := reflect.MakeSlice(v.Type(), v.Len(), v.Cap())
		for i := 0; i < v.Len(); i++ {
			elemVal, err := EvalObject(ctx, v.Index(i).Interface(), vars)
			if err != nil {
				return nil, err
			}
			newSlice.Index(i).Set(reflect.ValueOf(elemVal))
		}
		return newSlice.Interface(), nil
	default:
		// For other types, we can just return the object as is
		return obj, nil
	}
}

// processMap recursively processes a map, evaluating string values and recursively processing
// nested maps and structs.
func processMapWithVars(ctx context.Context, v reflect.Value, vars map[string]string) (reflect.Value, error) {
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
			evalResult, err := EvalString(ctx, strVal, WithVariables(vars))
			if err != nil {
				return v, err
			}
			newVal = reflect.ValueOf(evalResult)
		case reflect.Map:
			// Recursively process nested maps
			newVal, err = processMapWithVars(ctx, val, vars)
			if err != nil {
				return v, err
			}
		case reflect.Slice, reflect.Array:
			// Process slices and arrays by evaluating each element
			newSlice := reflect.MakeSlice(val.Type(), val.Len(), val.Cap())
			for i := 0; i < val.Len(); i++ {
				newVal, err := EvalObject(ctx, val.Index(i).Interface(), vars)
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
			evalOpts := WithVariables(vars)
			processed, err := EvalStringFields(ctx, structVal, evalOpts)
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
