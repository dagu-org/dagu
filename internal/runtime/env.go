package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/common/collections"
	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/common/mailer"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
)

// Env holds information about the DAG and the current step to execute
// including the variables (environment variables and DAG variables) that are
// available to the step.
type Env struct {
	// Embedded execution metadata from parent DAG run containing DAGRunID,
	// RootDAGRun reference, DAG configuration, database interface,
	// DAG-level environment variables, and coordinator dispatcher
	Context

	// Thread-safe map storing output variables from previously executed steps
	// in the format "key=value". These variables are populated when a step
	// completes and has an Output field defined, making the step's stdout
	// available to subsequent steps via variable substitution
	Variables *collections.SyncMap

	// The current step being executed within this environment context
	Step core.Step

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

// AllEnvs returns all environment variables that needs to be passed to the command.
func (e Env) AllEnvs() []string {
	envs := e.Context.AllEnvs()
	for k, v := range e.Envs {
		envs = append(envs, k+"="+v)
	}
	e.Variables.Range(func(_, value any) bool {
		envs = append(envs, value.(string))
		return true
	})
	return envs
}

// UserEnvsMap returns user-defined environment variables as a map,
// excluding OS environment (BaseEnv). Use this for isolated execution environments.
// Precedence: Step.Env > Envs > Variables > SecretEnvs > DAGContext.Envs > DAG.Env
func (e Env) UserEnvsMap() map[string]string {
	result := e.Context.UserEnvsMap() // DAG-level + secrets, no OS env

	// Add variables from previous steps
	e.Variables.Range(func(_, value any) bool {
		key, val, found := strings.Cut(value.(string), "=")
		if found {
			result[key] = val
		}
		return true
	})

	// Add step-specific envs (PWD, DAG_RUN_STEP_NAME, etc)
	for k, v := range e.Envs {
		result[k] = v
	}

	// Add step-defined env only if not already set by evaluated Variables.
	// Variables contains evaluated values (e.g., secrets expanded), while Step.Env
	// contains raw values (e.g., "${MY_SECRET}"). We don't want to overwrite
	// the evaluated values with raw placeholders.
	for _, env := range e.Step.Env {
		key, value, found := strings.Cut(env, "=")
		if !found {
			continue
		}
		if _, exists := result[key]; exists {
			continue
		}
		result[key] = value
	}

	return result
}

// NewEnv creates a new Env configured for executing the provided step.
// It resolves the step's working directory and sets initial per-step environment
// variables: PWD to the resolved working directory and the DAG run step name.
// The returned Env embeds the DAG context from ctx, stores the provided step,
// initializes an empty StepMap, and populates Variables from DAG.Params: for each
// param containing "=", the text before the first "=" is used as the key and the
// entire param string is stored as the value.
func NewEnv(ctx context.Context, step core.Step) Env {
	rCtx := GetDAGContext(ctx)
	workingDir := resolveWorkingDir(ctx, step, rCtx)

	envs := map[string]string{
		exec.EnvKeyDAGRunStepName: step.Name,
		"PWD":                     workingDir,
	}

	variables := &collections.SyncMap{}
	if rCtx.DAG != nil {
		for _, param := range rCtx.DAG.Params {
			key, _, found := strings.Cut(param, "=")
			if found {
				variables.Store(key, param)
			}
		}
	}

	return Env{
		Context:    rCtx,
		Variables:  variables,
		Step:       step,
		Envs:       envs,
		StepMap:    make(map[string]cmdutil.StepInfo),
		WorkingDir: workingDir,
	}
}

func resolveWorkingDir(ctx context.Context, step core.Step, rCtx Context) string {
	dag := rCtx.DAG

	if step.Dir != "" {
		// Expand environment variables in step.Dir using DAG env vars
		// Since we no longer use os.Setenv, we need to manually expand using dag.Env
		expandedDir := os.Expand(step.Dir, func(key string) string {
			// Check DAG-level env vars
			if dag != nil {
				for _, env := range dag.Env {
					if len(env) > len(key)+1 && env[:len(key)] == key && env[len(key)] == '=' {
						return env[len(key)+1:]
					}
				}
			}
			// Fall back to process environment
			return os.Getenv(key)
		})

		if filepath.IsAbs(expandedDir) || strings.HasPrefix(expandedDir, "~") {
			dir, err := fileutil.ResolvePath(expandedDir)
			if err != nil {
				logger.Warn(ctx, "Failed to resolve working directory for step",
					tag.Step(step.Name),
					tag.Dir(expandedDir),
					tag.Error(err),
				)
			}
			return dir
		} else if dag != nil && dag.WorkingDir != "" {
			// use relative path to the DAG's working dir
			return filepath.Clean(filepath.Join(dag.WorkingDir, expandedDir))
		} else {
			// This should not happen normally
			logger.Warn(ctx, "Failed to resolve working directory for step",
				tag.Step(step.Name),
				tag.Dir(expandedDir),
			)
			return expandedDir
		}

	}

	// Use the DAG level working directory if not specified
	if dag != nil && dag.WorkingDir != "" {
		return dag.WorkingDir
	}

	// This should not occur on normal execution
	logger.Warn(ctx, "Failed to resolve working directory for step",
		tag.Step(step.Name),
	)

	if wd, err := os.Getwd(); err == nil {
		return wd
	} else {
		logger.Error(ctx, "Failed to get current working directory",
			tag.Error(err),
		)
	}

	// If still empty, fallback to home directory
	dir, err := os.UserHomeDir()
	if err != nil {
		logger.Error(ctx, "Failed to get user home directory",
			tag.Error(err),
		)
	}
	return dir
}

// Shell returns the shell command to use for this execution context.
func (e Env) Shell(ctx context.Context) []string {
	// Shell precedence: Step shell -> DAG shell -> Global default
	if e.Step.Shell != "" {
		shellCmd, err := e.EvalString(ctx, e.Step.Shell)
		if err != nil {
			logger.Error(ctx, "Failed to evaluate step shell",
				tag.String("shell", e.Step.Shell),
				tag.Error(err),
			)
			return nil
		}
		shell := []string{shellCmd}
		for _, arg := range e.Step.ShellArgs {
			evaluated, err := e.EvalString(ctx, arg)
			if err != nil {
				logger.Error(ctx, "Failed to evaluate step shell argument",
					tag.String("arg", arg),
					tag.Error(err),
				)
			}
			shell = append(shell, evaluated)
		}
		return shell
	}

	if e.DAG.Shell != "" {
		return append([]string{e.DAG.Shell}, e.DAG.ShellArgs...)
	}

	shellCmd := cmdutil.GetShellCommand("")
	if shellCmd != "" {
		return []string{shellCmd}
	}
	logger.Debug(ctx, "Global default shell is not set or could not be determined")
	return nil
}

// DAGRunRef returns the DAGRunRef for the current execution context.
func (e Env) DAGRunRef() exec.DAGRunRef {
	return exec.NewDAGRunRef(e.DAG.Name, e.DAGRunID)
}

// LoadOutputVariables loads the output variables from the given DAG into the
func (e Env) LoadOutputVariables(vars *collections.SyncMap) {
	e.loadOutputVariables(vars, false)
}

// ForceLoadOutputVariables forces loading of output variables into the execution context.
// This is the same as LoadOutputVariables, but it does not check if the key already exists.
func (e Env) ForceLoadOutputVariables(vars *collections.SyncMap) {
	e.loadOutputVariables(vars, true)
}

// loadOutputVariables loads the output variables from the given SyncMap into the execution context.
// If force is true, it will overwrite existing variables in the execution context.
// If force is false, it will only load variables that are not already present in the execution context.
func (e Env) loadOutputVariables(vars *collections.SyncMap, force bool) {
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
	option := cmdutil.NewEvalOptions()
	for _, opt := range opts {
		opt(option)
	}

	// Collect environment variables for evaluating the string.
	// Variables are processed sequentially, and once a variable is replaced,
	// it cannot be overridden by subsequent maps.
	// Therefore, the effective precedence (highest to lowest) is:
	// 1. Step level environment variables (e.Envs) - processed first, highest precedence
	// 2. Output variables from previous steps (e.Variables) - processed second
	// 3. Secrets (dagEnv.SecretEnvs) - processed third
	// 4. DAG level environment variables (dagEnv.Envs) - processed fourth
	// 5. Additional options passed as parameters - processed last
	//
	// Example: If step env has FOO="step" and DAG env has FOO="dag",
	// ${FOO} will be replaced with "step" in the first iteration,
	// leaving no ${FOO} for the DAG env to replace.

	rCtx := GetDAGContext(ctx)
	if option.ExpandEnv {
		opts = append(opts, cmdutil.WithVariables(e.Envs))
		opts = append(opts, cmdutil.WithVariables(e.Variables.Variables()))
		opts = append(opts, cmdutil.WithVariables(rCtx.SecretEnvs))
		opts = append(opts, cmdutil.WithVariables(rCtx.Envs))
	} else {
		opts = append(opts, cmdutil.WithVariables(e.Envs))
		opts = append(opts, cmdutil.WithVariables(e.Variables.Variables()))
	}

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

// WithEnvVars returns a new execution context with the given environment variable(s).
func (e Env) WithEnvVars(envs ...string) Env {
	if len(envs)%2 != 0 {
		panic("invalid number of arguments")
	}
	for i := 0; i < len(envs); i += 2 {
		e.Envs[envs[i]] = envs[i+1]
	}
	return e
}

// WithVariables returns a new execution context with the given variable(s).
func (e Env) WithVariables(vars ...string) Env {
	if len(vars)%2 != 0 {
		panic("invalid number of arguments")
	}
	for i := 0; i < len(vars); i += 2 {
		e.Variables.Store(vars[i], vars[i]+"="+vars[i+1])
	}
	return e
}

// Context key for storing Env in context
type envCtxKey struct{}

// WithEnv returns a new context with the given execution context.
func WithEnv(ctx context.Context, e Env) context.Context {
	return context.WithValue(ctx, envCtxKey{}, e)
}

// GetEnv returns the execution context from the given context.
func GetEnv(ctx context.Context) Env {
	v, ok := ctx.Value(envCtxKey{}).(Env)
	if !ok {
		return NewEnv(ctx, core.Step{})
	}
	return v
}

// AllEnvs returns all environment variables that needs to be passed to the command.
// Each element is in the form of "key=value".
func AllEnvs(ctx context.Context) []string {
	return GetEnv(ctx).AllEnvs()
}

// AllEnvsMap builds a map of environment variables from the current Env.
// It splits each "key=value" entry produced by AllEnvs and maps keys to values;
// entries that do not contain an "=" separator are ignored.
func AllEnvsMap(ctx context.Context) map[string]string {
	envs := GetEnv(ctx).AllEnvs()
	var result = make(map[string]string)
	for _, env := range envs {
		key, value, found := strings.Cut(env, "=")
		if found {
			result[key] = value
		}
	}
	return result
}
