package scheduler

import (
	"github.com/dagu-org/dagu/internal/digraph"
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
func WithRootDagRun(ref digraph.DAGRunRef) TaskOption {
	return func(task *coordinatorv1.Task) {
		if ref.Name == "" || ref.ID == "" {
			return // No root DAG run reference provided
		}
		task.RootDagRunName = ref.Name
		task.RootDagRunId = ref.ID
	}
}

// WithParentDagRun sets the parent DAG run name and ID in the task.
func WithParentDagRun(ref digraph.DAGRunRef) TaskOption {
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
