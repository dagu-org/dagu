package digraph

const SystemVariablePrefix = "DAGU_"

// Special environment variables.
const (
	EnvKeyLogPath          = "DAG_EXECUTION_LOG_PATH"
	EnvKeySchedulerLogPath = "DAG_SCHEDULER_LOG_PATH" // Deprecated in favor of EnvKeyDAGStepLogPath
	EnvKeyExecID           = "DAG_REQUEST_ID"
	EnvKeyDAGName          = "DAG_NAME"
	EnvKeyStepName         = "DAG_STEP_NAME"
	EnvKeyStepLogPath      = "DAG_STEP_LOG_PATH"
)
