package convert

import (
	"testing"

	"github.com/dagu-org/dagu/internal/common/collections"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDAGRunStatusToProto(t *testing.T) {
	t.Run("nil status", func(t *testing.T) {
		result := DAGRunStatusToProto(nil)
		assert.Nil(t, result)
	})

	t.Run("basic status", func(t *testing.T) {
		status := &exec.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
		}

		result := DAGRunStatusToProto(status)
		require.NotNil(t, result)
		assert.NotEmpty(t, result.JsonData)
	})
}

func TestProtoToDAGRunStatus(t *testing.T) {
	t.Run("nil proto", func(t *testing.T) {
		result := ProtoToDAGRunStatus(nil)
		assert.Nil(t, result)
	})

	t.Run("empty json_data", func(t *testing.T) {
		proto := &coordinatorv1.DAGRunStatusProto{JsonData: ""}
		result := ProtoToDAGRunStatus(proto)
		assert.Nil(t, result)
	})
}

func TestRoundTrip(t *testing.T) {
	t.Run("full status roundtrip", func(t *testing.T) {
		outputVars := &collections.SyncMap{}
		outputVars.Store("key1", "value1")
		outputVars.Store("key2", "value2")

		original := &exec.DAGRunStatus{
			Name:       "test-dag",
			DAGRunID:   "run-123",
			AttemptID:  "attempt-1",
			Status:     core.Running,
			WorkerID:   "worker-1",
			PID:        12345,
			Root:       exec.DAGRunRef{Name: "root-dag", ID: "root-run"},
			Parent:     exec.DAGRunRef{Name: "parent-dag", ID: "parent-run"},
			CreatedAt:  1234567890,
			QueuedAt:   "2024-01-01T00:00:00Z",
			StartedAt:  "2024-01-01T00:01:00Z",
			FinishedAt: "2024-01-01T00:02:00Z",
			Log:        "/path/to/log",
			Error:      "test error",
			Params:     "key=value",
			ParamsList: []string{"key=value"},
			Nodes: []*exec.Node{
				{
					Step: core.Step{
						Name:           "step-1",
						Description:    "first step",
						ExecutorConfig: core.ExecutorConfig{Type: "shell"},
					},
					Status:          core.NodeSucceeded,
					Stdout:          "/path/stdout.log",
					Stderr:          "/path/stderr.log",
					StartedAt:       "2024-01-01T00:00:00Z",
					FinishedAt:      "2024-01-01T00:01:00Z",
					Error:           "step error",
					RetryCount:      2,
					DoneCount:       3,
					RetriedAt:       "2024-01-01T00:00:30Z",
					OutputVariables: outputVars,
					SubRuns: []exec.SubDAGRun{
						{DAGRunID: "sub-run-1", Params: "p1=v1"},
						{DAGRunID: "sub-run-2", Params: "p2=v2"},
					},
				},
			},
			OnInit:    &exec.Node{Step: core.Step{Name: "on-init"}, Status: core.NodeSucceeded},
			OnExit:    &exec.Node{Step: core.Step{Name: "on-exit"}, Status: core.NodeSucceeded},
			OnSuccess: &exec.Node{Step: core.Step{Name: "on-success"}, Status: core.NodeSucceeded},
			OnFailure: &exec.Node{Step: core.Step{Name: "on-failure"}, Status: core.NodeNotStarted},
			OnCancel:  &exec.Node{Step: core.Step{Name: "on-cancel"}, Status: core.NodeNotStarted},
			OnWait:    &exec.Node{Step: core.Step{Name: "on-wait"}, Status: core.NodeNotStarted},
		}

		// Convert to proto and back
		proto := DAGRunStatusToProto(original)
		result := ProtoToDAGRunStatus(proto)

		// Verify all fields are preserved
		require.NotNil(t, result)
		assert.Equal(t, original.Name, result.Name)
		assert.Equal(t, original.DAGRunID, result.DAGRunID)
		assert.Equal(t, original.AttemptID, result.AttemptID)
		assert.Equal(t, original.Status, result.Status)
		assert.Equal(t, original.WorkerID, result.WorkerID)
		assert.Equal(t, original.PID, result.PID)
		assert.Equal(t, original.Root.Name, result.Root.Name)
		assert.Equal(t, original.Root.ID, result.Root.ID)
		assert.Equal(t, original.Parent.Name, result.Parent.Name)
		assert.Equal(t, original.Parent.ID, result.Parent.ID)
		assert.Equal(t, original.CreatedAt, result.CreatedAt)
		assert.Equal(t, original.QueuedAt, result.QueuedAt)
		assert.Equal(t, original.StartedAt, result.StartedAt)
		assert.Equal(t, original.FinishedAt, result.FinishedAt)
		assert.Equal(t, original.Log, result.Log)
		assert.Equal(t, original.Error, result.Error)
		assert.Equal(t, original.Params, result.Params)
		assert.Equal(t, original.ParamsList, result.ParamsList)

		// Verify nodes
		require.Len(t, result.Nodes, 1)
		node := result.Nodes[0]
		assert.Equal(t, "step-1", node.Step.Name)
		assert.Equal(t, "first step", node.Step.Description)
		assert.Equal(t, "shell", node.Step.ExecutorConfig.Type)
		assert.Equal(t, core.NodeSucceeded, node.Status)
		assert.Equal(t, "/path/stdout.log", node.Stdout)
		assert.Equal(t, "/path/stderr.log", node.Stderr)
		assert.Equal(t, 2, node.RetryCount)
		assert.Equal(t, 3, node.DoneCount)
		require.Len(t, node.SubRuns, 2)
		assert.Equal(t, "sub-run-1", node.SubRuns[0].DAGRunID)
		assert.Equal(t, "sub-run-2", node.SubRuns[1].DAGRunID)

		// Verify handler nodes
		require.NotNil(t, result.OnInit)
		assert.Equal(t, "on-init", result.OnInit.Step.Name)
		require.NotNil(t, result.OnExit)
		assert.Equal(t, "on-exit", result.OnExit.Step.Name)
		require.NotNil(t, result.OnSuccess)
		assert.Equal(t, "on-success", result.OnSuccess.Step.Name)
		require.NotNil(t, result.OnFailure)
		assert.Equal(t, "on-failure", result.OnFailure.Step.Name)
		require.NotNil(t, result.OnCancel)
		assert.Equal(t, "on-cancel", result.OnCancel.Step.Name)
		require.NotNil(t, result.OnWait)
		assert.Equal(t, "on-wait", result.OnWait.Step.Name)
	})
}
