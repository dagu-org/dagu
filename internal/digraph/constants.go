package digraph

const SystemVariablePrefix = "DAGU_"

// Special environment variables.
const (
	EnvKeyWorkflowID             = "WORKFLOW_ID"
	EnvKeyWorkflowName           = "WORKFLOW_NAME"
	EnvKeyWorkflowLogFile        = "WORKFLOW_LOG_FILE"
	EnvKeyWorkflowStepName       = "WORKFLOW_STEP_NAME"
	EnvKeyWorkflowStepStdoutFile = "WORKFLOW_STEP_STDOUT_FILE"
	EnvKeyWorkflowStepStderrFile = "WORKFLOW_STEP_STDERR_FILE"
)
