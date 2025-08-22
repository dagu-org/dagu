package digraph

import (
	"errors"
	"fmt"
	"strings"
)

// LoadError represents an error in a specific field of the configuration
type LoadError struct {
	Field string
	Value any
	Err   error
}

func (e *LoadError) Error() string {
	if e.Value == nil {
		return fmt.Sprintf("field '%s': %v", e.Field, e.Err)
	}
	return fmt.Sprintf("field '%s': %v (value: %+v)", e.Field, e.Err, e.Value)
}

func (e *LoadError) Unwrap() error {
	return e.Err
}

// wrapError wraps an error with field context
func wrapError(field string, value any, err error) error {
	return &LoadError{
		Field: field,
		Value: value,
		Err:   err,
	}
}

// errors on building a DAG.
var (
	ErrNameTooLong                         = errors.New("name must be less than 40 characters")
	ErrNameInvalidChars                    = errors.New("name must only contain alphanumeric characters, dashes, dots, and underscores")
	ErrInvalidSchedule                     = errors.New("invalid schedule")
	ErrScheduleMustBeStringOrArray         = errors.New("schedule must be a string or an array of strings")
	ErrInvalidScheduleType                 = errors.New("invalid schedule type")
	ErrInvalidKeyType                      = errors.New("invalid key type")
	ErrExecutorConfigMustBeString          = errors.New("executor config key must be string")
	ErrDuplicateFunction                   = errors.New("duplicate function")
	ErrFuncParamsMismatch                  = errors.New("func params and args given to func command do not match")
	ErrInvalidStepData                     = errors.New("invalid step data")
	ErrStepNameRequired                    = errors.New("step name must be specified")
	ErrStepNameDuplicate                   = errors.New("step name must be unique")
	ErrStepNameTooLong                     = errors.New("step name must be less than 40 characters")
	ErrStepCommandIsRequired               = errors.New("step command is required")
	ErrStepCommandIsEmpty                  = errors.New("step command is empty")
	ErrStepCommandMustBeArrayOrString      = errors.New("step command must be an array of strings or a string")
	ErrInvalidParamValue                   = errors.New("invalid parameter value")
	ErrCallFunctionNotFound                = errors.New("call must specify a functions that exists")
	ErrNumberOfParamsMismatch              = errors.New("the number of parameters defined in the function does not match the number of parameters given")
	ErrRequiredParameterNotFound           = errors.New("required parameter not found")
	ErrScheduleKeyMustBeString             = errors.New("schedule key must be a string")
	ErrInvalidSignal                       = errors.New("invalid signal")
	ErrInvalidEnvValue                     = errors.New("invalid value for env")
	ErrArgsMustBeConvertibleToIntOrString  = errors.New("args must be convertible to either int or string")
	ErrExecutorTypeMustBeString            = errors.New("executor.type value must be string")
	ErrExecutorConfigValueMustBeMap        = errors.New("executor.config value must be a map")
	ErrExecutorHasInvalidKey               = errors.New("executor has invalid key")
	ErrExecutorConfigMustBeStringOrMap     = errors.New("executor config must be string or map")
	ErrDotEnvMustBeStringOrArray           = errors.New("dotenv must be a string or an array of strings")
	ErrPreconditionMustBeArrayOrString     = errors.New("precondition must be a string or an array of strings")
	ErrPreconditionValueMustBeString       = errors.New("precondition value must be a string")
	ErrPreconditionHasInvalidKey           = errors.New("precondition has invalid key")
	ErrContinueOnOutputMustBeStringOrArray = errors.New("continueOn.Output must be a string or an array of strings")
	ErrContinueOnExitCodeMustBeIntOrArray  = errors.New("continueOn.ExitCode must be an int or an array of ints")
	ErrDependsMustBeStringOrArray          = errors.New("depends must be a string or an array of strings")
	ErrStepsMustBeArrayOrMap               = errors.New("steps must be an array or a map")
)

// ErrorList is just a list of errors.
// It is used to collect multiple errors in building a DAG.
type ErrorList []error

// Add adds an error to the list.
func (e *ErrorList) Add(err error) {
	if err != nil {
		*e = append(*e, err)
	}
}

// ToStringList returns the list of errors as a slice of strings.
func (e *ErrorList) ToStringList() []string {
	errStrings := make([]string, len(*e))
	for i, err := range *e {
		errStrings[i] = err.Error()
	}
	return errStrings
}

// Error implements the error interface.
// It returns a string with all the errors separated by a semicolon.
func (e ErrorList) Error() string {
	errStrings := make([]string, len(e))
	for i, err := range e {
		errStrings[i] = err.Error()
	}
	return strings.Join(errStrings, "; ")
}

// Unwrap implements the errors.Unwrap interface.
func (e ErrorList) Unwrap() []error {
	// If the list is empty, return nil
	if len(e) == 0 {
		return nil
	}

	// Return a copy of the underlying error slice
	// This allows errors.Is to check against each error in the list
	return e
}
