package spec

import (
	"context"
	"fmt"
	"strings"

	"github.com/dagu-org/dagu/internal/cmn/eval"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/go-viper/mapstructure/v2"
)

// BuildContext is the context for building a DAG.
type BuildContext struct {
	ctx   context.Context
	file  string
	opts  BuildOpts
	index int

	// buildEnv is a temporary map used during core.DAG building to pass env vars to params
	// This is not serialized and is cleared after build completes
	buildEnv map[string]string

	// envScope is a shared state pointer for thread-safe environment variable handling.
	// It holds accumulated env vars (OS + DAG env) and is used by transformers
	// to expand variables without mutating global os.Env.
	// This is initialized by build() and populated by buildEnvs transformer.
	envScope *envScopeState
}

// envScopeState holds mutable state that needs to be shared across transformers.
// Using a pointer allows value-passed BuildContext to share state.
type envScopeState struct {
	scope    *eval.EnvScope
	buildEnv map[string]string // Also store as map for WithVariables
}

// StepBuildContext is the context for building a step.
type StepBuildContext struct {
	BuildContext
	dag *core.DAG
}

func (c BuildContext) WithOpts(opts BuildOpts) BuildContext {
	copy := c
	copy.opts = opts
	return copy
}

func (c BuildContext) WithFile(file string) BuildContext {
	copy := c
	copy.file = file
	return copy
}

// BuildFlag represents a bitmask option that influences DAG building behaviour.
type BuildFlag uint32

const (
	BuildFlagNone BuildFlag = 0

	BuildFlagNoEval BuildFlag = 1 << iota
	BuildFlagOnlyMetadata
	BuildFlagAllowBuildErrors
	BuildFlagSkipSchemaValidation
	BuildFlagSkipBaseHandlers // Skip merging handlerOn from base config (for sub-DAG runs)
)

// BuildOpts is used to control the behavior of the builder.
type BuildOpts struct {
	// Base specifies the Base configuration file for the DAG.
	Base string
	// Parameters specifies the Parameters to the DAG.
	// Parameters are used to override the default Parameters in the DAG.
	Parameters string
	// ParametersList specifies the parameters to the DAG.
	ParametersList []string
	// Name of the core.DAG if it's not defined in the spec
	Name string
	// DAGsDir is the directory containing the core.DAG files.
	DAGsDir string
	// DefaultWorkingDir is the default working directory for DAGs without explicit workingDir.
	// This is used for sub-DAG execution to inherit the parent's working directory.
	DefaultWorkingDir string
	// Flags stores all boolean options controlling build behaviour.
	Flags BuildFlag
	// BuildEnv provides pre-populated environment variables for the build.
	// These are added to envScope before building, allowing YAML to reference
	// them via ${VAR}. Used for retry/restart where dotenv values need to be
	// available during rebuild from YamlData.
	BuildEnv map[string]string
}

// Has reports whether the flag is enabled on the current BuildOpts.
func (o BuildOpts) Has(flag BuildFlag) bool {
	return o.Flags&flag != 0
}

// parsePrecondition parses the precondition field.
func parsePrecondition(ctx BuildContext, precondition any) ([]*core.Condition, error) {
	switch v := precondition.(type) {
	case nil:
		return nil, nil

	case string:
		return []*core.Condition{{Condition: v}}, nil

	case map[string]any:
		var ret core.Condition
		for key, vv := range v {
			switch strings.ToLower(key) {
			case "condition":
				val, ok := vv.(string)
				if !ok {
					return nil, core.NewValidationError("preconditions", vv, ErrPreconditionValueMustBeString)
				}
				ret.Condition = val

			case "expected":
				val, ok := vv.(string)
				if !ok {
					return nil, core.NewValidationError("preconditions", vv, ErrPreconditionValueMustBeString)
				}
				ret.Expected = val

			case "command":
				val, ok := vv.(string)
				if !ok {
					return nil, core.NewValidationError("preconditions", vv, ErrPreconditionValueMustBeString)
				}
				ret.Condition = val

			case "negate":
				val, ok := vv.(bool)
				if !ok {
					return nil, core.NewValidationError("preconditions", vv, ErrPreconditionNegateMustBeBool)
				}
				ret.Negate = val

			default:
				return nil, core.NewValidationError("preconditions", key, fmt.Errorf("%w: %s", ErrPreconditionHasInvalidKey, key))

			}
		}

		if err := ret.Validate(); err != nil {
			return nil, core.NewValidationError("preconditions", v, err)
		}

		return []*core.Condition{&ret}, nil

	case []any:
		var ret []*core.Condition
		for _, vv := range v {
			parsed, err := parsePrecondition(ctx, vv)
			if err != nil {
				return nil, err
			}
			ret = append(ret, parsed...)
		}
		return ret, nil

	default:
		return nil, core.NewValidationError("preconditions", v, ErrPreconditionMustBeArrayOrString)

	}
}

// parseSecretRefs parses secret references from the YAML definition.
func parseSecretRefs(secretRefs []secretRef) ([]core.SecretRef, error) {

	// Convert secretRef to core.SecretRef and validate
	secrets := make([]core.SecretRef, 0, len(secretRefs))
	names := make(map[string]bool)

	for i, def := range secretRefs {
		// Validate required fields
		if def.Name == "" {
			return nil, core.NewValidationError("secrets", def, fmt.Errorf("secret at index %d: 'name' field is required", i))
		}
		if def.Provider == "" {
			return nil, core.NewValidationError("secrets", def, fmt.Errorf("secret at index %d: 'provider' field is required", i))
		}
		if def.Key == "" {
			return nil, core.NewValidationError("secrets", def, fmt.Errorf("secret at index %d: 'key' field is required", i))
		}

		// Check for duplicate names
		if names[def.Name] {
			return nil, core.NewValidationError("secrets", def, fmt.Errorf("duplicate secret name %q", def.Name))
		}
		names[def.Name] = true

		secrets = append(secrets, core.SecretRef{
			Name:     def.Name,
			Provider: def.Provider,
			Key:      def.Key,
			Options:  def.Options,
		})
	}

	return secrets, nil
}

// generateTypedStepName generates a type-based name for a step after it's been built
func generateTypedStepName(existingNames map[string]struct{}, step *core.Step, index int) string {
	var prefix string

	// Determine prefix based on the built step's properties
	if step.ExecutorConfig.Type != "" {
		prefix = step.ExecutorConfig.Type
	} else if step.Container != nil {
		prefix = "docker"
	} else if step.Parallel != nil {
		prefix = "parallel"
	} else if step.SubDAG != nil {
		prefix = "dag"
	} else if step.Script != "" {
		prefix = "script"
	} else if len(step.Commands) > 0 {
		prefix = "cmd"
	} else {
		prefix = "step"
	}

	// Generate unique name with the prefix
	counter := index + 1
	name := fmt.Sprintf("%s_%d", prefix, counter)

	for {
		if _, exists := existingNames[name]; !exists {
			existingNames[name] = struct{}{}
			return name
		}
		counter++
		name = fmt.Sprintf("%s_%d", prefix, counter)
	}
}

// normalizedStepData converts string to map[string]any for subsequent process
func normalizeStepData(ctx BuildContext, data []any) []any {
	// Convert string steps to map format for shorthand syntax support
	normalized := make([]any, len(data))
	for i, item := range data {
		switch step := item.(type) {
		case string:
			// Shorthand: convert string to map with command field
			normalized[i] = map[string]any{"command": step}
		default:
			// Keep as-is (already a map or other structure)
			normalized[i] = item
		}
	}
	return normalized
}

// buildStepFromRaw build core.Step from give raw data (map[string]any)
func buildStepFromRaw(ctx StepBuildContext, idx int, raw map[string]any, names map[string]struct{}, defs *defaults) (*core.Step, error) {
	var st step
	md, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		ErrorUnused: true,
		Result:      &st,
		TagName:     "yaml",
		DecodeHook:  TypedUnionDecodeHook(),
	})
	if err := md.Decode(raw); err != nil {
		return nil, core.NewValidationError("steps", raw, withSnakeCaseKeyHint(err))
	}
	applyDefaults(&st, defs, raw)
	builtStep, err := st.build(ctx)
	if err != nil {
		return nil, err
	}
	if builtStep.Name == "" {
		builtStep.Name = generateTypedStepName(names, builtStep, idx)
	}
	return builtStep, nil
}

// injectChainDependencies adds implicit dependencies for chain type execution.
// In chain execution, each step depends on the immediately previous step(s).
func injectChainDependencies(dag *core.DAG, prevSteps []*core.Step, step *core.Step) {
	// Early returns for cases where we shouldn't inject dependencies
	if dag.Type != core.TypeChain || len(prevSteps) == 0 {
		return
	}

	// Build a set of existing dependencies for efficient lookup
	existingDeps := make(map[string]struct{}, len(step.Depends))
	for _, dep := range step.Depends {
		existingDeps[dep] = struct{}{}
	}

	// Add each previous step as a dependency if not already present
	for _, prevStep := range prevSteps {
		var depKey = prevStep.ID
		if depKey == "" {
			depKey = prevStep.Name
		}

		// Skip if this dependency already exists
		if _, exists := existingDeps[depKey]; exists {
			continue
		}

		// Also check alternate key (ID vs Name) to avoid duplicates
		altKey := getStepAlternateKey(prevStep, depKey)
		if altKey != "" {
			if _, exists := existingDeps[altKey]; exists {
				continue
			}
		}

		step.Depends = append(step.Depends, depKey)
		existingDeps[depKey] = struct{}{}
	}
}

// getStepAlternateKey returns the alternate identifier for a step, or empty string if none
func getStepAlternateKey(step *core.Step, primaryKey string) string {
	if step.ID != "" && primaryKey == step.ID {
		return step.Name
	}
	if step.ID != "" && primaryKey == step.Name {
		return step.ID
	}
	return ""
}
