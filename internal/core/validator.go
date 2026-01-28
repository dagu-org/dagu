package core

import (
	"fmt"
	"regexp"
	"strings"
)

// DAGNameMaxLen defines the maximum allowed length for a DAG name.
const DAGNameMaxLen = 40

// dagNameRegex matches valid DAG names: alphanumeric, underscore, dash, dot.
var dagNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

// ValidateDAGName validates a DAG name according to shared rules.
// - Empty name is allowed (caller may provide one via context or filename).
// - Non-empty name must satisfy length and allowed character constraints.
func ValidateDAGName(name string) error {
	if name == "" {
		return nil
	}
	if len(name) > DAGNameMaxLen {
		return ErrNameTooLong
	}
	if !dagNameRegex.MatchString(name) {
		return ErrNameInvalidChars
	}
	return nil
}

// StepValidator is a function type for validating step configurations.
type StepValidator func(step Step) error

var stepValidators = make(map[string]StepValidator)

func RegisterStepValidator(executorType string, validator StepValidator) {
	stepValidators[executorType] = validator
}

// ValidateSteps exposes validateSteps for packages that need to perform validation during DAG construction.
// It collects all validation errors instead of returning on first error.
func ValidateSteps(dag *DAG) error {
	var errs ErrorList

	// First pass: collect all names and IDs
	stepNames := make(map[string]struct{})
	stepIDs := make(map[string]struct{})

	for _, step := range dag.Steps {
		// Names should always exist at this point (explicit or auto-generated)
		if step.Name == "" {
			// This should not happen if generation works correctly
			errs = append(errs, NewValidationError("steps", step, fmt.Errorf("internal error: step name not generated")))
			continue
		}

		if _, exists := stepNames[step.Name]; exists {
			errs = append(errs, NewValidationError("steps", step.Name, ErrStepNameDuplicate))
		} else {
			stepNames[step.Name] = struct{}{}
		}

		// Collect IDs if present
		if step.ID != "" {
			// Check ID format
			if !isValidStepID(step.ID) {
				errs = append(errs, NewValidationError("steps", step.ID, fmt.Errorf("invalid step ID format: must match pattern ^[a-zA-Z][a-zA-Z0-9_-]*$")))
			}

			// Check for duplicate IDs
			if _, exists := stepIDs[step.ID]; exists {
				errs = append(errs, NewValidationError("steps", step.ID, fmt.Errorf("duplicate step ID: %s", step.ID)))
			} else {
				stepIDs[step.ID] = struct{}{}
			}

			// Check for reserved words
			if isReservedWord(step.ID) {
				errs = append(errs, NewValidationError("steps", step.ID, fmt.Errorf("step ID '%s' is a reserved word", step.ID)))
			}
		}
	}

	// Second pass: check for conflicts between names and IDs
	for _, step := range dag.Steps {
		if step.Name == "" {
			continue // Skip steps with empty names (already reported in first pass)
		}

		if step.ID != "" {
			// Check that ID doesn't conflict with any step name
			if _, exists := stepNames[step.ID]; exists && step.ID != step.Name {
				errs = append(errs, NewValidationError("steps", step.ID, fmt.Errorf("step ID '%s' conflicts with another step's name", step.ID)))
			}
		}

		// Check that name doesn't conflict with any ID (unless it's the same step)
		if _, exists := stepIDs[step.Name]; exists {
			// Find if this is the same step
			sameStep := false
			for _, s := range dag.Steps {
				if s.Name == step.Name && s.ID == step.Name {
					sameStep = true
					break
				}
			}
			if !sameStep {
				errs = append(errs, NewValidationError("steps", step.Name, fmt.Errorf("step name '%s' conflicts with another step's ID", step.Name)))
			}
		}
	}

	// Third pass: resolve step IDs to names in depends fields
	if err := resolveStepDependencies(dag); err != nil {
		errs = append(errs, err)
	}

	// Fourth pass: validate dependencies exist
	for _, step := range dag.Steps {
		for _, dep := range step.Depends {
			if _, exists := stepNames[dep]; !exists {
				errs = append(errs, NewValidationError("depends", dep, fmt.Errorf("step %s depends on non-existent step %s", step.Name, dep)))
			}
		}
	}

	// Final pass: validate each step
	for _, step := range dag.Steps {
		errs = append(errs, validateStep(step)...)
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}

func validateStep(step Step) ErrorList {
	var errs ErrorList

	if step.Name == "" {
		errs = append(errs, NewValidationError("name", step.Name, ErrStepNameRequired))
	}

	if len(step.Name) > maxStepNameLen {
		errs = append(errs, NewValidationError("name", step.Name, ErrStepNameTooLong))
	}

	if step.Parallel != nil {
		// Parallel steps must have a run field (child-DAG only for MVP)
		if step.SubDAG == nil {
			errs = append(errs, NewValidationError("parallel", step.Parallel, fmt.Errorf("parallel execution is only supported for child-DAGs (must have 'run' field)")))
		}

		// MaxConcurrent must be positive
		if step.Parallel.MaxConcurrent <= 0 {
			errs = append(errs, NewValidationError("parallel.maxConcurrent", step.Parallel.MaxConcurrent, fmt.Errorf("maxConcurrent must be greater than 0")))
		}

		// Must have either items or variable reference
		if len(step.Parallel.Items) == 0 && step.Parallel.Variable == "" {
			errs = append(errs, NewValidationError("parallel", step.Parallel, fmt.Errorf("parallel must have either items array or variable reference")))
		}
	}

	// Validate executor-specific configuration if validator exists
	if err := validateStepWithValidator(step); err != nil {
		errs = append(errs, err)
	}

	return errs
}

func validateStepWithValidator(step Step) error {
	validator, exists := stepValidators[step.ExecutorConfig.Type]
	if !exists || validator == nil {
		// No validator registered for this executor type
		return nil
	}
	if err := validator(step); err != nil {
		return NewValidationError("executorConfig", step.ExecutorConfig, err)
	}
	return nil
}

// maxStepNameLen is the maximum length of a step name.
const maxStepNameLen = 40

// stepIDPattern defines the valid format for step IDs.
var stepIDPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// isValidStepID checks if the given ID matches the required pattern.
func isValidStepID(id string) bool {
	return stepIDPattern.MatchString(id)
}

// isReservedWord checks if the given ID is a reserved word.
func isReservedWord(id string) bool {
	reservedWords := map[string]bool{
		"env":     true,
		"params":  true,
		"args":    true,
		"stdout":  true,
		"stderr":  true,
		"output":  true,
		"outputs": true,
	}
	return reservedWords[strings.ToLower(id)]
}

// resolveStepDependencies resolves step IDs to step names in the depends field.
func resolveStepDependencies(dag *DAG) error {
	idToName := make(map[string]string)
	for i := range dag.Steps {
		step := &dag.Steps[i]
		if step.ID != "" {
			idToName[step.ID] = step.Name
		}
	}

	for i := range dag.Steps {
		step := &dag.Steps[i]
		for j, dep := range step.Depends {
			if name, exists := idToName[dep]; exists {
				step.Depends[j] = name
			}
		}
	}

	return nil
}
