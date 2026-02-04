package cmdutil

import (
	"context"
	"fmt"
	"reflect"
)

// EvalObject recursively evaluates the string fields of the given object.
func EvalObject[T any](ctx context.Context, obj T, vars map[string]string, opts ...EvalOption) (T, error) {
	result, err := evalObject(ctx, obj, vars, opts...)
	if err != nil {
		return obj, err
	}
	if v, ok := result.(T); ok {
		return v, nil
	}
	return obj, fmt.Errorf("expected type %T but got %T", obj, result)
}

// evalObject evaluates the given object based on its type.
func evalObject(ctx context.Context, obj any, vars map[string]string, opts ...EvalOption) (any, error) {
	v := reflect.ValueOf(obj)

	allOpts := make([]EvalOption, 0, len(opts)+1)
	allOpts = append(allOpts, WithVariables(vars))
	allOpts = append(allOpts, opts...)

	// nolint:exhaustive
	switch v.Kind() {
	case reflect.String:
		return EvalString(ctx, v.String(), allOpts...)

	case reflect.Struct:
		return EvalStringFields(ctx, obj, allOpts...)

	case reflect.Map:
		result, err := processMapWithVars(ctx, v, vars, opts...)
		if err != nil {
			return nil, err
		}
		return result.Interface(), nil

	case reflect.Slice, reflect.Array:
		newSlice := reflect.MakeSlice(v.Type(), v.Len(), v.Cap())
		for i := 0; i < v.Len(); i++ {
			elemVal, err := EvalObject(ctx, v.Index(i).Interface(), vars, opts...)
			if err != nil {
				return nil, err
			}
			newSlice.Index(i).Set(reflect.ValueOf(elemVal))
		}
		return newSlice.Interface(), nil

	default:
		return obj, nil
	}
}

// processMapWithVars recursively processes a map, evaluating string values and
// recursively processing nested maps and structs.
func processMapWithVars(ctx context.Context, v reflect.Value, vars map[string]string, opts ...EvalOption) (reflect.Value, error) {
	newMap := reflect.MakeMap(v.Type())

	allOpts := make([]EvalOption, 0, len(opts)+1)
	allOpts = append(allOpts, WithVariables(vars))
	allOpts = append(allOpts, opts...)

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
			evalResult, err := EvalString(ctx, val.String(), allOpts...)
			if err != nil {
				return v, err
			}
			newVal = reflect.ValueOf(evalResult)

		case reflect.Map:
			newVal, err = processMapWithVars(ctx, val, vars, opts...)
			if err != nil {
				return v, err
			}

		case reflect.Slice, reflect.Array:
			newSlice := reflect.MakeSlice(val.Type(), val.Len(), val.Cap())
			for i := 0; i < val.Len(); i++ {
				elemVal, err := EvalObject(ctx, val.Index(i).Interface(), vars, opts...)
				if err != nil {
					return v, err
				}
				newSlice.Index(i).Set(reflect.ValueOf(elemVal))
			}
			newVal = newSlice

		case reflect.Struct:
			processed, err := EvalStringFields(ctx, val.Interface(), allOpts...)
			if err != nil {
				return v, err
			}
			newVal = reflect.ValueOf(processed)

		default:
			newVal = val
		}

		newMap.SetMapIndex(key, newVal)
	}

	return newMap, nil
}
