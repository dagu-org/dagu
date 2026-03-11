// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package executor

import (
	"log/slog"
	"os"

	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/proto/convert"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// CreateTask creates a coordinator task from this DAG for distributed execution.
// It constructs a task with the given operation and run ID, setting the DAG's name
// as both the root DAG and target, and includes the DAG's YAML definition.
func CreateTask(
	dagName string,
	yamlDefinition string,
	op coordinatorv1.Operation,
	runID string,
	opts ...TaskOption,
) *coordinatorv1.Task {
	task := &coordinatorv1.Task{
		RootDagRunName: dagName,
		RootDagRunId:   runID,
		Operation:      op,
		DagRunId:       runID,
		Target:         dagName,
		Definition:     yamlDefinition,
	}

	for _, opt := range opts {
		opt(task)
	}

	return task
}

// TaskOption is a function that modifies a coordinatorv1.Task.
type TaskOption func(*coordinatorv1.Task)

// WithRootDagRun sets the root DAG run name and ID in the task.
func WithRootDagRun(ref exec.DAGRunRef) TaskOption {
	return func(task *coordinatorv1.Task) {
		if ref.Name == "" || ref.ID == "" {
			return // No root DAG run reference provided
		}
		task.RootDagRunName = ref.Name
		task.RootDagRunId = ref.ID
	}
}

// WithParentDagRun sets the parent DAG run name and ID in the task.
func WithParentDagRun(ref exec.DAGRunRef) TaskOption {
	return func(task *coordinatorv1.Task) {
		if ref.Name == "" || ref.ID == "" {
			return // No parent DAG run reference provided
		}
		task.ParentDagRunName = ref.Name
		task.ParentDagRunId = ref.ID
	}
}

// WithTaskParams sets the parameters for the task.
func WithTaskParams(params string) TaskOption {
	return func(task *coordinatorv1.Task) {
		task.Params = params
	}
}

// WithWorkerSelector sets the worker selector labels for the task.
func WithWorkerSelector(selector map[string]string) TaskOption {
	return func(task *coordinatorv1.Task) {
		task.WorkerSelector = selector
	}
}

// WithStep sets the step name for retry operations.
func WithStep(step string) TaskOption {
	return func(task *coordinatorv1.Task) {
		task.Step = step
	}
}

// WithTags sets additional tags (comma-separated) for the task.
func WithTags(tags string) TaskOption {
	return func(task *coordinatorv1.Task) {
		task.Tags = tags
	}
}

// WithTaskTriggerType sets the trigger type on the task.
func WithTaskTriggerType(triggerType string) TaskOption {
	return func(task *coordinatorv1.Task) {
		task.TriggerType = triggerType
	}
}

// WithTaskScheduledTime sets the scheduled time on the task.
func WithTaskScheduledTime(scheduledTime string) TaskOption {
	return func(task *coordinatorv1.Task) {
		task.ScheduledTime = scheduledTime
	}
}

// WithBaseConfig sets the base config YAML content on the task.
// This allows workers to apply base config without needing local base config files.
func WithBaseConfig(content string) TaskOption {
	return func(task *coordinatorv1.Task) {
		task.BaseConfig = content
	}
}

// ResolveBaseConfig returns the base config content for a DAG task.
// It prefers embedded BaseConfigData from the DAG, falling back to reading the file at fallbackPath.
func ResolveBaseConfig(baseConfigData []byte, fallbackPath string) string {
	if len(baseConfigData) > 0 {
		return string(baseConfigData)
	}
	if fallbackPath == "" {
		return ""
	}
	data, err := os.ReadFile(fallbackPath) //nolint:gosec
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Debug("failed to read base config file", "path", fallbackPath, "error", err)
		}
		return ""
	}
	return string(data)
}

// WithPreviousStatus sets the previous status for retry operations in shared-nothing mode.
// When set, workers can retry without needing local DAGRunStore access.
func WithPreviousStatus(status *exec.DAGRunStatus) TaskOption {
	return func(task *coordinatorv1.Task) {
		if status != nil {
			protoStatus, err := convert.DAGRunStatusToProto(status)
			if err != nil {
				slog.Error("failed to convert previous status to proto", "error", err)
				return
			}
			task.PreviousStatus = protoStatus
		}
	}
}
