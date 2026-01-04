package transform_test

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/common/collections"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/runtime/transform"
	"github.com/stretchr/testify/assert"
)

func TestNodeFieldsRoundTrip(t *testing.T) {
	outputVars := &collections.SyncMap{}
	outputVars.Store("KEY", "KEY=value")

	original := &execution.Node{
		Step:            core.Step{Name: "test-step"},
		Status:          core.NodeSucceeded,
		Stdout:          "/tmp/stdout.log",
		Stderr:          "/tmp/stderr.log",
		StartedAt:       "2024-01-15T10:00:00Z",
		FinishedAt:      "2024-01-15T10:05:00Z",
		RetriedAt:       "2024-01-15T10:01:00Z",
		RetryCount:      2,
		DoneCount:       3,
		Repeated:        true,
		Error:           "test error",
		SubRuns:         []execution.SubDAGRun{{DAGRunID: "sub-1", Params: "p1"}},
		SubRunsRepeated: []execution.SubDAGRun{{DAGRunID: "sub-2", Params: "p2"}},
		OutputVariables: outputVars,
		ApprovalInputs:  map[string]string{"input1": "value1"},
		ApprovedAt:      "2024-01-15T10:02:00Z",
		ApprovedBy:      "admin",
	}

	// Round-trip: execution.Node -> runtime.Node -> execution.Node
	runtimeNode := transform.ToNode(original)
	state := runtimeNode.State()

	dag := &core.DAG{Name: "test", Steps: []core.Step{original.Step}}
	status := transform.NewStatusBuilder(dag).Create("run-1", core.Succeeded, 0, time.Now(),
		transform.WithNodes([]runtime.NodeData{{Step: original.Step, State: state}}))

	result := status.Nodes[0]

	// OutputVariables is a pointer, compare separately
	assert.Equal(t, original.OutputVariables, result.OutputVariables)

	// Compare rest of the struct
	original.OutputVariables = nil
	result.OutputVariables = nil
	assert.Equal(t, original, result)
}
