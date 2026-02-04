package eval

import (
	"context"
	"fmt"
	"reflect"
)

// Object recursively evaluates the string fields of the given object.
// It uses variable expansion, command substitution, and basic env expansion
// (via evalStringValue), preserving undefined variables as-is.
func Object[T any](ctx context.Context, obj T, vars map[string]string) (T, error) {
	options := NewOptions()
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
