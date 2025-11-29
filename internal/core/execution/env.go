package execution

// Environment variable keys that are automatically set by Dagu during execution.
const (
	// EnvKeyDAGName holds the name of the currently executing DAG.
	EnvKeyDAGName = "DAG_NAME"

	// EnvKeyDAGRunID holds the unique identifier for the current DAG run.
	EnvKeyDAGRunID = "DAG_RUN_ID"

	// EnvKeyDAGRunLogFile holds the path to the main log file for the DAG run.
	EnvKeyDAGRunLogFile = "DAG_RUN_LOG_FILE"

	// EnvKeyDAGRunStepName holds the name of the currently executing step.
	EnvKeyDAGRunStepName = "DAG_RUN_STEP_NAME"

	// EnvKeyDAGRunStepStdoutFile holds the path to the stdout log file for the current step.
	EnvKeyDAGRunStepStdoutFile = "DAG_RUN_STEP_STDOUT_FILE"

	// EnvKeyDAGRunStepStderrFile holds the path to the stderr log file for the current step.
	EnvKeyDAGRunStepStderrFile = "DAG_RUN_STEP_STDERR_FILE"

	// EnvKeyDAGRunStatus holds the current status of the DAG run (e.g., "running", "success", "failed").
	EnvKeyDAGRunStatus = "DAG_RUN_STATUS"
)
