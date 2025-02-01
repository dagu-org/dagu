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
	errInvalidSchedule                     = errors.New("invalid schedule")
	errScheduleMustBeStringOrArray         = errors.New("schedule must be a string or an array of strings")
	errInvalidScheduleType                 = errors.New("invalid schedule type")
	errInvalidKeyType                      = errors.New("invalid key type")
	errExecutorConfigMustBeString          = errors.New("executor config key must be string")
	errDuplicateFunction                   = errors.New("duplicate function")
	errFuncParamsMismatch                  = errors.New("func params and args given to func command do not match")
	errStepNameRequired                    = errors.New("step name must be specified")
	errStepCommandIsRequired               = errors.New("step command is required")
	errStepCommandIsEmpty                  = errors.New("step command is empty")
	errStepCommandMustBeArrayOrString      = errors.New("step command must be an array of strings or a string")
	errInvalidParamValue                   = errors.New("invalid parameter value")
	errCallFunctionNotFound                = errors.New("call must specify a functions that exists")
	errNumberOfParamsMismatch              = errors.New("the number of parameters defined in the function does not match the number of parameters given")
	errRequiredParameterNotFound           = errors.New("required parameter not found")
	errScheduleKeyMustBeString             = errors.New("schedule key must be a string")
	errInvalidSignal                       = errors.New("invalid signal")
	errInvalidEnvValue                     = errors.New("invalid value for env")
	errArgsMustBeConvertibleToIntOrString  = errors.New("args must be convertible to either int or string")
	errExecutorTypeMustBeString            = errors.New("executor.type value must be string")
	errExecutorConfigValueMustBeMap        = errors.New("executor.config value must be a map")
	errExecutorHasInvalidKey               = errors.New("executor has invalid key")
	errExecutorConfigMustBeStringOrMap     = errors.New("executor config must be string or map")
	errDotenvMustBeStringOrArray           = errors.New("dotenv must be a string or an array of strings")
	errPreconditionMustBeArrayOrString     = errors.New("precondition must be a string or an array of strings")
	errPreconditionKeyMustBeString         = errors.New("precondition key must be a string")
	errPreconditionValueMustBeString       = errors.New("precondition value must be a string")
	errPreconditionHasInvalidKey           = errors.New("precondition has invalid key")
	errContinueOnOutputMustBeStringOrArray = errors.New("continueOn.Output must be a string or an array of strings")
	errContinueOnExitCodeMustBeIntOrArray  = errors.New("continueOn.ExitCode must be an int or an array of ints")
	errDependsMustBeStringOrArray          = errors.New("depends must be a string or an array of strings")
	errStepsMustBeArrayOrMap               = errors.New("steps must be an array or a map")
)

// errorList is just a list of errors.
// It is used to collect multiple errors in building a DAG.
type errorList []error

// Add adds an error to the list.
func (e *errorList) Add(err error) {
	if err != nil {
		*e = append(*e, err)
	}
}

// Error implements the error interface.
// It returns a string with all the errors separated by a semicolon.
func (e *errorList) Error() string {
	errStrings := make([]string, len(*e))
	for i, err := range *e {
		errStrings[i] = err.Error()
	}
	return strings.Join(errStrings, "; ")
}
