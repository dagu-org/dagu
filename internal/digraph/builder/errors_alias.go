package builder

import digraph "github.com/dagu-org/dagu/internal/digraph"

var (
	ErrInvalidSchedule                     = digraph.ErrInvalidSchedule
	ErrScheduleMustBeStringOrArray         = digraph.ErrScheduleMustBeStringOrArray
	ErrInvalidScheduleType                 = digraph.ErrInvalidScheduleType
	ErrDotEnvMustBeStringOrArray           = digraph.ErrDotEnvMustBeStringOrArray
	ErrPreconditionValueMustBeString       = digraph.ErrPreconditionValueMustBeString
	ErrPreconditionHasInvalidKey           = digraph.ErrPreconditionHasInvalidKey
	ErrPreconditionMustBeArrayOrString     = digraph.ErrPreconditionMustBeArrayOrString
	ErrInvalidStepData                     = digraph.ErrInvalidStepData
	ErrStepsMustBeArrayOrMap               = digraph.ErrStepsMustBeArrayOrMap
	ErrContinueOnExitCodeMustBeIntOrArray  = digraph.ErrContinueOnExitCodeMustBeIntOrArray
	ErrContinueOnOutputMustBeStringOrArray = digraph.ErrContinueOnOutputMustBeStringOrArray
	ErrInvalidSignal                       = digraph.ErrInvalidSignal
	ErrDependsMustBeStringOrArray          = digraph.ErrDependsMustBeStringOrArray
	ErrExecutorTypeMustBeString            = digraph.ErrExecutorTypeMustBeString
	ErrExecutorConfigValueMustBeMap        = digraph.ErrExecutorConfigValueMustBeMap
	ErrExecutorHasInvalidKey               = digraph.ErrExecutorHasInvalidKey
	ErrExecutorConfigMustBeStringOrMap     = digraph.ErrExecutorConfigMustBeStringOrMap
	ErrInvalidEnvValue                     = digraph.ErrInvalidEnvValue
	ErrInvalidParamValue                   = digraph.ErrInvalidParamValue
	ErrStepCommandIsEmpty                  = digraph.ErrStepCommandIsEmpty
	ErrStepCommandMustBeArrayOrString      = digraph.ErrStepCommandMustBeArrayOrString
	ErrStepCommandIsRequired               = digraph.ErrStepCommandIsRequired
)
