package builder

import "errors"

var (
	ErrInvalidSchedule                     = errors.New("invalid schedule")
	ErrScheduleMustBeStringOrArray         = errors.New("schedule must be a string or an array of strings")
	ErrInvalidScheduleType                 = errors.New("invalid schedule type")
	ErrDotEnvMustBeStringOrArray           = errors.New("dotenv must be a string or an array of strings")
	ErrPreconditionValueMustBeString       = errors.New("precondition value must be a string")
	ErrPreconditionHasInvalidKey           = errors.New("precondition has invalid key")
	ErrPreconditionMustBeArrayOrString     = errors.New("precondition must be a string or an array of strings")
	ErrInvalidStepData                     = errors.New("invalid step data")
	ErrStepsMustBeArrayOrMap               = errors.New("steps must be an array or a map")
	ErrContinueOnExitCodeMustBeIntOrArray  = errors.New("continueOn.ExitCode must be an int or an array of ints")
	ErrContinueOnOutputMustBeStringOrArray = errors.New("continueOn.Output must be a string or an array of strings")
	ErrInvalidSignal                       = errors.New("invalid signal")
	ErrDependsMustBeStringOrArray          = errors.New("depends must be a string or an array of strings")
	ErrExecutorTypeMustBeString            = errors.New("executor.type value must be string")
	ErrExecutorConfigValueMustBeMap        = errors.New("executor.config value must be a map")
	ErrExecutorHasInvalidKey               = errors.New("executor has invalid key")
	ErrExecutorConfigMustBeStringOrMap     = errors.New("executor config must be string or map")
	ErrInvalidEnvValue                     = errors.New("env config should be map of strings or array of key=value formatted string")
	ErrInvalidParamValue                   = errors.New("invalid parameter value")
	ErrStepCommandIsEmpty                  = errors.New("step command is empty")
	ErrStepCommandMustBeArrayOrString      = errors.New("step command must be an array of strings or a string")
	ErrStepCommandIsRequired               = errors.New("step command is required")
)
