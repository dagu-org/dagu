package digraph

import (
	"context"
	"fmt"
	"strconv"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/mailer"
)

type StepContext struct {
	Context
	outputVariables *SyncMap
	step            Step
	envs            map[string]string
}

func NewStepContext(ctx context.Context, step Step) StepContext {
	return StepContext{
		Context: GetContext(ctx),

		outputVariables: &SyncMap{},
		step:            step,
		envs: map[string]string{
			EnvKeyDAGStepName: step.Name,
		},
	}
}

func (c StepContext) AllEnvs() []string {
	envs := c.Context.AllEnvs()
	for k, v := range c.envs {
		envs = append(envs, k+"="+v)
	}
	c.outputVariables.Range(func(_, value any) bool {
		envs = append(envs, value.(string))
		return true
	})
	return envs
}

func (c StepContext) LoadOutputVariables(vars *SyncMap) {
	vars.Range(func(key, value any) bool {
		// Skip if the key already exists
		if _, ok := c.outputVariables.Load(key); ok {
			return true
		}
		c.outputVariables.Store(key, value)
		return true
	})
}

func (c StepContext) MailerConfig() (mailer.Config, error) {
	return EvalStringFields(c, mailer.Config{
		Host:     c.dag.SMTP.Host,
		Port:     c.dag.SMTP.Port,
		Username: c.dag.SMTP.Username,
		Password: c.dag.SMTP.Password,
	})
}

func (c StepContext) EvalString(s string, opts ...cmdutil.EvalOption) (string, error) {
	opts = append(opts, cmdutil.WithVariables(c.envs))
	opts = append(opts, cmdutil.WithVariables(c.outputVariables.Variables()))
	return cmdutil.EvalString(c.ctx, s, opts...)
}

func (c StepContext) EvalBool(value any) (bool, error) {
	switch v := value.(type) {
	case string:
		s, err := c.EvalString(v)
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

func (c StepContext) WithEnv(key, value string) StepContext {
	c.envs[key] = value
	return c
}

func WithStepContext(ctx context.Context, stepContext StepContext) context.Context {
	return context.WithValue(ctx, stepCtxKey{}, stepContext)
}

func GetStepContext(ctx context.Context) StepContext {
	contextValue, ok := ctx.Value(stepCtxKey{}).(StepContext)
	if !ok {
		return NewStepContext(ctx, Step{})
	}
	return contextValue
}

func IsStepContext(ctx context.Context) bool {
	_, ok := ctx.Value(stepCtxKey{}).(StepContext)
	return ok
}

type stepCtxKey struct{}

func EvalStringFields[T any](stepContext StepContext, obj T) (T, error) {
	return cmdutil.EvalStringFields(stepContext.ctx, obj,
		cmdutil.WithVariables(stepContext.outputVariables.Variables()))
}
