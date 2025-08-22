package executor

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/mailer"
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
	// Embedded execution metadata from parent DAG run containing DAGRunID,
	// RootDAGRun reference, DAG configuration, database interface,
	// DAG-level environment variables, and coordinator dispatcher
	digraph.Env

	// Thread-safe map storing output variables from previously executed steps
	// in the format "key=value". These variables are populated when a step
	// completes and has an Output field defined, making the step's stdout
	// available to subsequent steps via variable substitution
	Variables *SyncMap

	// The current step being executed within this environment context
	Step digraph.Step

	// Additional environment variables specific to this step execution,
	// including DAG_RUN_STEP_NAME and PWD. These take precedence over
	// Variables and DAG-level Envs during variable evaluation
	Envs map[string]string

	// Maps step IDs to their execution information (stdout, stderr, exitCode)
	// allowing steps to reference outputs from other steps using expressions
	// like ${stepID.stdout} or ${stepID.exitCode} in their configurations
	StepMap map[string]cmdutil.StepInfo

	// Resolved absolute path for the step's working directory, determined by:
	// 1. Step's Dir field if specified (resolved to absolute path)
	// 2. Current working directory if Dir is not specified
	// This path is also set as the PWD environment variable
	WorkingDir string
}

func (e Env) VariablesMap() map[string]string {
	m := e.Variables.Variables()
	for k, v := range e.Envs {
		m[k] = v
	}
	return m
}

// NewEnv creates a new execution context with the given step.
func NewEnv(ctx context.Context, step digraph.Step) Env {
	parentEnv := digraph.GetEnv(ctx)
	parentDAG := parentEnv.DAG

	var workingDir string

	switch {
	case step.Dir != "":
		dir, err := fileutil.ResolvePath(step.Dir)
		if err == nil {
			workingDir = dir
		} else {
			logger.Warn(ctx, "Failed to resolve working directory for step", "step", step.Name, "dir", dir, "err", err)
			workingDir = parentEnv.DAG.WorkingDir
		}

	case parentDAG != nil && parentDAG.WorkingDir != "":
		workingDir = parentDAG.WorkingDir

	default:
		// Use the current working directory if not specified
		if wd, err := os.Getwd(); err == nil {
			workingDir = wd
		} else {
			logger.Error(ctx, "Failed to get current working directory", "err", err)
		}
	}

	envs := map[string]string{
		digraph.EnvKeyDAGRunStepName: step.Name,
	}

	if workingDir != "" {
		envs["PWD"] = workingDir
	}

	return Env{
		Env:        digraph.GetEnv(ctx),
		Variables:  &SyncMap{},
		Step:       step,
		Envs:       envs,
		StepMap:    make(map[string]cmdutil.StepInfo),
		WorkingDir: workingDir,
	}
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
	dagEnv := digraph.GetEnv(ctx)

	// Collect environment variables for evaluating the string.
	// Variables are processed sequentially, and once a variable is replaced,
	// it cannot be overridden by subsequent maps.
	// Therefore, the effective precedence (highest to lowest) is:
	// 1. Step level environment variables (e.Envs) - processed first, highest precedence
	// 2. Output variables from previous steps (e.Variables) - processed second
	// 3. DAG level environment variables (dagEnv.Envs) - processed third
	// 4. Additional options passed as parameters - processed last
	//
	// Example: If step env has FOO="step" and DAG env has FOO="dag",
	// ${FOO} will be replaced with "step" in the first iteration,
	// leaving no ${FOO} for the DAG env to replace.

	opts = append(opts, cmdutil.WithVariables(e.Envs))
	opts = append(opts, cmdutil.WithVariables(e.Variables.Variables()))
	opts = append(opts, cmdutil.WithVariables(dagEnv.Envs))

	// Step data for special variables such as step ID and exit code
	opts = append(opts, cmdutil.WithStepMap(e.StepMap))

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
