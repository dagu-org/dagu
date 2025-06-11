package executor

import (
	"context"
	"fmt"
	"strconv"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/mailer"
	"github.com/dagu-org/dagu/internal/resource"
)

// AllEnvs returns all environment variables that needs to be passed to the command.
// Each element is in the form of "key=value".
func AllEnvs(ctx context.Context) []string {
	return GetEnv(ctx).AllEnvs()
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

	Variables          *SyncMap
	Step               digraph.Step
	Envs               map[string]string
	ResourceController *resource.ResourceController
}

// NewEnv creates a new execution context with the given step.
func NewEnv(ctx context.Context, step digraph.Step) Env {
	env := Env{
		Env:       digraph.GetEnv(ctx),
		Variables: &SyncMap{},
		Step:      step,
		Envs: map[string]string{
			digraph.EnvKeyDAGRunStepName: step.Name,
		},
	}
	
	// Get resource controller from context if available
	if rc := digraph.GetResourceController(ctx); rc != nil {
		if resourceController, ok := rc.(*resource.ResourceController); ok {
			env.ResourceController = resourceController
		}
	}
	
	return env
}

// DAGRunRef returns the DAGRunRef for the current execution context.
func (e Env) DAGRunRef() digraph.DAGRunRef {
	return digraph.NewDAGRunRef(e.DAG.Name, e.DAGRunID)
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
	e.loadOutputVariables(vars, false)
}

// ForceLoadOutputVariables forces loading of output variables into the execution context.
// This is the same as LoadOutputVariables, but it does not check if the key already exists.
func (e Env) ForceLoadOutputVariables(vars *SyncMap) {
	e.loadOutputVariables(vars, true)
}

// loadOutputVariables loads the output variables from the given SyncMap into the execution context.
// If force is true, it will overwrite existing variables in the execution context.
// If force is false, it will only load variables that are not already present in the execution context.
func (e Env) loadOutputVariables(vars *SyncMap, force bool) {
	vars.Range(func(key, value any) bool {
		if !force {
			if _, ok := e.Variables.Load(key); ok {
				return true
			}
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
