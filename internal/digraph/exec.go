package digraph

import (
	"context"
	"fmt"
	"strconv"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/mailer"
)

// AllEnvs returns all environment variables that needs to be passed to the command.
// Each element is in the form of "key=value".
func AllEnvs(ctx context.Context) []string {
	return GetExecContext(ctx).AllEnvs()
}

// EvalString evaluates the given string with the variables within the execution context.
func EvalString(ctx context.Context, s string, opts ...cmdutil.EvalOption) (string, error) {
	return GetExecContext(ctx).EvalString(ctx, s, opts...)
}

// EvalBool evaluates the given value with the variables within the execution context
// and parses it as a boolean.
func EvalBool(ctx context.Context, value any) (bool, error) {
	return GetExecContext(ctx).EvalBool(ctx, value)
}

// EvalObject recursively evaluates the string fields of the given object
// with the variables within the execution context.
func EvalObject[T any](ctx context.Context, obj T) (T, error) {
	vars := GetExecContext(ctx).vars.Variables()
	return cmdutil.EvalStringFields(ctx, obj, cmdutil.WithVariables(vars))
}

// WithExecContext returns a new context with the given execution context.
func WithExecContext(ctx context.Context, c ExecContext) context.Context {
	return context.WithValue(ctx, stepCtxKey{}, c)
}

// GetExecContext returns the execution context from the given context.
func GetExecContext(ctx context.Context) ExecContext {
	contextValue, ok := ctx.Value(stepCtxKey{}).(ExecContext)
	if !ok {
		return NewExecContext(ctx, Step{})
	}
	return contextValue
}

// ExecContext holds information about the DAG and the current step to execute
// including the variables (environment variables and DAG variables) that are
// available to the step.
type ExecContext struct {
	Context

	vars *SyncMap
	step Step
	envs map[string]string
}

func NewExecContext(ctx context.Context, step Step) ExecContext {
	return ExecContext{
		Context: GetContext(ctx),
		vars:    &SyncMap{},
		step:    step,
		envs: map[string]string{
			EnvKeyStepName: step.Name,
		},
	}
}

func (c ExecContext) AllEnvs() []string {
	envs := c.Context.AllEnvs()
	for k, v := range c.envs {
		envs = append(envs, k+"="+v)
	}
	c.vars.Range(func(_, value any) bool {
		envs = append(envs, value.(string))
		return true
	})
	return envs
}

func (c ExecContext) LoadOutputVariables(vars *SyncMap) {
	vars.Range(func(key, value any) bool {
		// Skip if the key already exists
		if _, ok := c.vars.Load(key); ok {
			return true
		}
		c.vars.Store(key, value)
		return true
	})
}

func (c ExecContext) MailerConfig(ctx context.Context) (mailer.Config, error) {
	if c.DAG.SMTP == nil {
		return mailer.Config{}, nil
	}
	return cmdutil.EvalStringFields(ctx, mailer.Config{
		Host:     c.DAG.SMTP.Host,
		Port:     c.DAG.SMTP.Port,
		Username: c.DAG.SMTP.Username,
		Password: c.DAG.SMTP.Password,
	}, cmdutil.WithVariables(c.vars.Variables()))
}

// EvalString evaluates the given string with the variables within the execution context.
func (c ExecContext) EvalString(ctx context.Context, s string, opts ...cmdutil.EvalOption) (string, error) {
	dagCtx := GetContext(ctx)
	opts = append(opts, cmdutil.WithVariables(dagCtx.Envs))
	opts = append(opts, cmdutil.WithVariables(c.envs))
	opts = append(opts, cmdutil.WithVariables(c.vars.Variables()))
	return cmdutil.EvalString(ctx, s, opts...)
}

// EvalBool evaluates the given value with the variables within the execution context
func (c ExecContext) EvalBool(ctx context.Context, value any) (bool, error) {
	switch v := value.(type) {
	case string:
		s, err := c.EvalString(ctx, v)
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
func (c ExecContext) WithEnv(envs ...string) ExecContext {
	if len(envs)%2 != 0 {
		panic("invalid number of arguments")
	}
	for i := 0; i < len(envs); i += 2 {
		c.envs[envs[i]] = envs[i+1]
	}
	return c
}

type stepCtxKey struct{}
