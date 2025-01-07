// Copyright (C) 2025 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package digraph

const SystemVariablePrefix = "DAGU_"

// Special environment variables.
const (
	EnvKeyLogPath          = "DAG_EXECUTION_LOG_PATH"
	EnvKeySchedulerLogPath = "DAG_SCHEDULER_LOG_PATH" // Deprecated in favor of EnvKeyDAGStepLogPath
	EnvKeyRequestID        = "DAG_REQUEST_ID"
	EnvKeyDAGName          = "DAG_NAME"
	EnvKeyDAGStepName      = "DAG_STEP_NAME"
	EnvKeyDAGStepLogPath   = "DAG_STEP_LOG_PATH"
)
