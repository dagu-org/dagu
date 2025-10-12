package core

// SystemVariablePrefix is the prefix for temporary variables used internally by Dagu
// to avoid conflicts with user-defined variables.
const SystemVariablePrefix = "DAGU_"

// Special environment variables.
const (
	EnvKeyDAGName              = "DAG_NAME"
	EnvKeyDAGRunID             = "DAG_RUN_ID"
	EnvKeyDAGRunLogFile        = "DAG_RUN_LOG_FILE"
	EnvKeyDAGRunStepName       = "DAG_RUN_STEP_NAME"
	EnvKeyDAGRunStepStdoutFile = "DAG_RUN_STEP_STDOUT_FILE"
	EnvKeyDAGRunStepStderrFile = "DAG_RUN_STEP_STDERR_FILE"
)
