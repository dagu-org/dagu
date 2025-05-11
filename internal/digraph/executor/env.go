package executor

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/mailer"
)

// AllEnvs returns all environment variables that needs to be passed to the command.
// Each element is in the form of "key=value".
func AllEnvs(ctx context.Context) []string {
	return GetEnv(ctx).AllEnvs()
}

// EvalString evaluates the given string with the variables within the execution context.
func EvalString(ctx context.Context, s string, opts ...cmdutil.EvalOption) (string, error) {
	return GetEnv(ctx).EvalString(ctx, s, opts...)
}

// EvalBool evaluates the given value with the variables within the execution context
// and parses it as a boolean.
func EvalBool(ctx context.Context, value any) (bool, error) {
	return GetEnv(ctx).EvalBool(ctx, value)
}

// EvalObject recursively evaluates the string fields of the given object
// with the variables within the execution context.
func EvalObject[T any](ctx context.Context, obj T) (T, error) {
	env := GetEnv(ctx).Variables.Variables()

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

// WithEnv returns a new context with the given execution context.
func WithEnv(ctx context.Context, e Env) context.Context {
	return context.WithValue(ctx, envCtxKey{}, e)
}

// GetEnv returns the execution context from the given context.
func GetEnv(ctx context.Context) Env {
	v, ok := ctx.Value(envCtxKey{}).(Env)
	if !ok {
		return NewEnv(ctx, digraph.Step{})
	}
	return v
}

// Env holds information about the DAG and the current step to execute
// including the variables (environment variables and DAG variables) that are
// available to the step.
type Env struct {
	digraph.Env

	Variables *SyncMap
	Step      digraph.Step
	Envs      map[string]string
}

// NewEnv creates a new execution context with the given step.
func NewEnv(ctx context.Context, step digraph.Step) Env {
	return Env{
		Env:       digraph.GetEnv(ctx),
		Variables: &SyncMap{},
		Step:      step,
		Envs: map[string]string{
			digraph.EnvKeyWorkflowStepName: step.Name,
		},
	}
}

// ExecRef returns the execution reference of the current execution context.
func (e Env) ExecRef() digraph.WorkflowRef {
	return digraph.NewWorkflowRef(e.DAG.Name, e.ExecID)
}

// AllEnvs returns all environment variables that needs to be passed to the command.
func (e Env) AllEnvs() []string {
	envs := e.Env.AllEnvs()
	for k, v := range e.Envs {
		envs = append(envs, k+"="+v)
	}
	e.Variables.Range(func(_, value any) bool {
		envs = append(envs, value.(string))
		return true
	})
	return envs
}

// LoadOutputVariables loads the output variables from the given DAG into the
func (e Env) LoadOutputVariables(vars *SyncMap) {
	vars.Range(func(key, value any) bool {
		// Skip if the key already exists
		if _, ok := e.Variables.Load(key); ok {
			return true
		}
		e.Variables.Store(key, value)
		return true
	})
}

func (e Env) MailerConfig(ctx context.Context) (mailer.Config, error) {
	if e.DAG.SMTP == nil {
		return mailer.Config{}, nil
	}
	return cmdutil.EvalStringFields(ctx, mailer.Config{
		Host:     e.DAG.SMTP.Host,
		Port:     e.DAG.SMTP.Port,
		Username: e.DAG.SMTP.Username,
		Password: e.DAG.SMTP.Password,
	}, cmdutil.WithVariables(e.Variables.Variables()))
}

// EvalString evaluates the given string with the variables within the execution context.
func (e Env) EvalString(ctx context.Context, s string, opts ...cmdutil.EvalOption) (string, error) {
	env := digraph.GetEnv(ctx)
	opts = append(opts, cmdutil.WithVariables(env.Envs))
	opts = append(opts, cmdutil.WithVariables(e.Envs))
	opts = append(opts, cmdutil.WithVariables(e.Variables.Variables()))
	return cmdutil.EvalString(ctx, s, opts...)
}

// EvalBool evaluates the given value with the variables within the execution context
func (e Env) EvalBool(ctx context.Context, value any) (bool, error) {
	switch v := value.(type) {
	case string:
		s, err := e.EvalString(ctx, v)
		if err != nil {
			return false, err
		}
		return strconv.ParseBool(s)
	case bool:
		return v, nil
	default:
		return false, fmt.Errorf("unsupported type %T for bool (value: %+v)", value, value)
	}
}

// WithEnv returns a new execution context with the given environment variable(s).
func (e Env) WithEnv(envs ...string) Env {
	if len(envs)%2 != 0 {
		panic("invalid number of arguments")
	}
	for i := 0; i < len(envs); i += 2 {
		e.Envs[envs[i]] = envs[i+1]
	}
	return e
}

type envCtxKey struct{}
