package convert

import (
	"testing"

	"github.com/dagu-org/dagu/internal/common/collections"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDAGRunStatusToProto(t *testing.T) {
	t.Run("nil status", func(t *testing.T) {
		result := DAGRunStatusToProto(nil)
		assert.Nil(t, result)
	})

	t.Run("basic fields", func(t *testing.T) {
		status := &execution.DAGRunStatus{
			Name:       "test-dag",
			DAGRunID:   "run-123",
			AttemptID:  "attempt-1",
			Status:     core.Running,
			WorkerID:   "worker-1",
			PID:        12345,
			CreatedAt:  1234567890,
			QueuedAt:   "2024-01-01T00:00:00Z",
			StartedAt:  "2024-01-01T00:01:00Z",
			FinishedAt: "2024-01-01T00:02:00Z",
			Log:        "/path/to/log",
			Error:      "test error",
			Params:     "key=value",
			ParamsList: []string{"key=value"},
		}

		result := DAGRunStatusToProto(status)
		require.NotNil(t, result)
		assert.Equal(t, "test-dag", result.Name)
		assert.Equal(t, "run-123", result.DagRunId)
		assert.Equal(t, "attempt-1", result.AttemptId)
		assert.Equal(t, int32(core.Running), result.Status)
		assert.Equal(t, "worker-1", result.WorkerId)
		assert.Equal(t, int32(12345), result.Pid)
		assert.Equal(t, int64(1234567890), result.CreatedAt)
		assert.Equal(t, "2024-01-01T00:00:00Z", result.QueuedAt)
		assert.Equal(t, "2024-01-01T00:01:00Z", result.StartedAt)
		assert.Equal(t, "2024-01-01T00:02:00Z", result.FinishedAt)
		assert.Equal(t, "/path/to/log", result.Log)
		assert.Equal(t, "test error", result.Error)
		assert.Equal(t, "key=value", result.Params)
		assert.Equal(t, []string{"key=value"}, result.ParamsList)
	})

	t.Run("with root and parent refs", func(t *testing.T) {
		status := &execution.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Root:     execution.DAGRunRef{Name: "root-dag", ID: "root-run"},
			Parent:   execution.DAGRunRef{Name: "parent-dag", ID: "parent-run"},
		}

		result := DAGRunStatusToProto(status)
		require.NotNil(t, result)
		require.NotNil(t, result.Root)
		assert.Equal(t, "root-dag", result.Root.Name)
		assert.Equal(t, "root-run", result.Root.Id)
		require.NotNil(t, result.Parent)
		assert.Equal(t, "parent-dag", result.Parent.Name)
		assert.Equal(t, "parent-run", result.Parent.Id)
	})

	t.Run("with nodes", func(t *testing.T) {
		status := &execution.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Nodes: []*execution.Node{
				{
					Step:       core.Step{Name: "step-1", Description: "first step", ExecutorConfig: core.ExecutorConfig{Type: "shell"}},
					Status:     core.NodeSucceeded,
					StartedAt:  "2024-01-01T00:00:00Z",
					FinishedAt: "2024-01-01T00:01:00Z",
				},
				{
					Step:   core.Step{Name: "step-2", Description: "second step", ExecutorConfig: core.ExecutorConfig{Type: "docker"}},
					Status: core.NodeRunning,
				},
			},
		}

		result := DAGRunStatusToProto(status)
		require.NotNil(t, result)
		require.Len(t, result.Nodes, 2)
		assert.Equal(t, "step-1", result.Nodes[0].StepName)
		assert.Equal(t, int32(core.NodeSucceeded), result.Nodes[0].Status)
		assert.Equal(t, "shell", result.Nodes[0].Step.ExecutorType)
		assert.Equal(t, "step-2", result.Nodes[1].StepName)
		assert.Equal(t, int32(core.NodeRunning), result.Nodes[1].Status)
		assert.Equal(t, "docker", result.Nodes[1].Step.ExecutorType)
	})

	t.Run("with handler nodes", func(t *testing.T) {
		status := &execution.DAGRunStatus{
			Name:      "test-dag",
			DAGRunID:  "run-123",
			OnInit:    &execution.Node{Step: core.Step{Name: "on-init"}, Status: core.NodeSucceeded},
			OnExit:    &execution.Node{Step: core.Step{Name: "on-exit"}, Status: core.NodeSucceeded},
			OnSuccess: &execution.Node{Step: core.Step{Name: "on-success"}, Status: core.NodeSucceeded},
			OnFailure: &execution.Node{Step: core.Step{Name: "on-failure"}, Status: core.NodeNotStarted},
			OnCancel:  &execution.Node{Step: core.Step{Name: "on-cancel"}, Status: core.NodeNotStarted},
			OnWait:    &execution.Node{Step: core.Step{Name: "on-wait"}, Status: core.NodeNotStarted},
		}

		result := DAGRunStatusToProto(status)
		require.NotNil(t, result)
		require.NotNil(t, result.OnInit)
		assert.Equal(t, "on-init", result.OnInit.StepName)
		require.NotNil(t, result.OnExit)
		assert.Equal(t, "on-exit", result.OnExit.StepName)
		require.NotNil(t, result.OnSuccess)
		assert.Equal(t, "on-success", result.OnSuccess.StepName)
		require.NotNil(t, result.OnFailure)
		assert.Equal(t, "on-failure", result.OnFailure.StepName)
		require.NotNil(t, result.OnCancel)
		assert.Equal(t, "on-cancel", result.OnCancel.StepName)
		require.NotNil(t, result.OnWait)
		assert.Equal(t, "on-wait", result.OnWait.StepName)
	})
}

func TestDAGRunRefToProto(t *testing.T) {
	t.Run("zero ref", func(t *testing.T) {
		result := DAGRunRefToProto(execution.DAGRunRef{})
		assert.Nil(t, result)
	})

	t.Run("valid ref", func(t *testing.T) {
		ref := execution.DAGRunRef{Name: "test-dag", ID: "run-123"}
		result := DAGRunRefToProto(ref)
		require.NotNil(t, result)
		assert.Equal(t, "test-dag", result.Name)
		assert.Equal(t, "run-123", result.Id)
	})
}

func TestNodeToProto(t *testing.T) {
	t.Run("nil node", func(t *testing.T) {
		result := NodeToProto(nil)
		assert.Nil(t, result)
	})

	t.Run("basic fields", func(t *testing.T) {
		node := &execution.Node{
			Step: core.Step{
				Name:           "test-step",
				Description:    "test description",
				ExecutorConfig: core.ExecutorConfig{Type: "shell"},
			},
			Status:     core.NodeSucceeded,
			Stdout:     "/path/stdout.log",
			Stderr:     "/path/stderr.log",
			StartedAt:  "2024-01-01T00:00:00Z",
			FinishedAt: "2024-01-01T00:01:00Z",
			Error:      "test error",
			RetryCount: 2,
			DoneCount:  3,
			RetriedAt:  "2024-01-01T00:00:30Z",
		}

		result := NodeToProto(node)
		require.NotNil(t, result)
		assert.Equal(t, "test-step", result.StepName)
		assert.Equal(t, int32(core.NodeSucceeded), result.Status)
		assert.Equal(t, "/path/stdout.log", result.Stdout)
		assert.Equal(t, "/path/stderr.log", result.Stderr)
		assert.Equal(t, "2024-01-01T00:00:00Z", result.StartedAt)
		assert.Equal(t, "2024-01-01T00:01:00Z", result.FinishedAt)
		assert.Equal(t, "test error", result.Error)
		assert.Equal(t, int32(2), result.RetryCount)
		assert.Equal(t, int32(3), result.DoneCount)
		assert.Equal(t, "2024-01-01T00:00:30Z", result.RetriedAt)

		// Verify Step proto includes ExecutorType
		require.NotNil(t, result.Step)
		assert.Equal(t, "test-step", result.Step.Name)
		assert.Equal(t, "test description", result.Step.Description)
		assert.Equal(t, "shell", result.Step.ExecutorType)
	})

	t.Run("with sub runs", func(t *testing.T) {
		node := &execution.Node{
			Step: core.Step{Name: "test-step"},
			SubRuns: []execution.SubDAGRun{
				{DAGRunID: "sub-run-1", Params: "p1=v1"},
				{DAGRunID: "sub-run-2", Params: "p2=v2"},
			},
		}

		result := NodeToProto(node)
		require.NotNil(t, result)
		require.Len(t, result.SubRuns, 2)
		assert.Equal(t, "sub-run-1", result.SubRuns[0].DagRunId)
		assert.Equal(t, "p1=v1", result.SubRuns[0].Params)
		assert.Equal(t, "sub-run-2", result.SubRuns[1].DagRunId)
		assert.Equal(t, "p2=v2", result.SubRuns[1].Params)
	})

	t.Run("with output variables", func(t *testing.T) {
		node := &execution.Node{
			Step:            core.Step{Name: "test-step"},
			OutputVariables: &collections.SyncMap{},
		}
		node.OutputVariables.Store("key1", "value1")
		node.OutputVariables.Store("key2", "value2")

		result := NodeToProto(node)
		require.NotNil(t, result)
		require.NotNil(t, result.OutputVariables)
		assert.Equal(t, "value1", result.OutputVariables["key1"])
		assert.Equal(t, "value2", result.OutputVariables["key2"])
	})
}

func TestProtoToDAGRunStatus(t *testing.T) {
	t.Run("nil proto", func(t *testing.T) {
		result := ProtoToDAGRunStatus(nil)
		assert.Nil(t, result)
	})

	t.Run("basic fields", func(t *testing.T) {
		proto := &coordinatorv1.DAGRunStatusProto{
			Name:       "test-dag",
			DagRunId:   "run-123",
			AttemptId:  "attempt-1",
			Status:     int32(core.Running),
			WorkerId:   "worker-1",
			Pid:        12345,
			CreatedAt:  1234567890,
			QueuedAt:   "2024-01-01T00:00:00Z",
			StartedAt:  "2024-01-01T00:01:00Z",
			FinishedAt: "2024-01-01T00:02:00Z",
			Log:        "/path/to/log",
			Error:      "test error",
			Params:     "key=value",
			ParamsList: []string{"key=value"},
		}

		result := ProtoToDAGRunStatus(proto)
		require.NotNil(t, result)
		assert.Equal(t, "test-dag", result.Name)
		assert.Equal(t, "run-123", result.DAGRunID)
		assert.Equal(t, "attempt-1", result.AttemptID)
		assert.Equal(t, core.Running, result.Status)
		assert.Equal(t, "worker-1", result.WorkerID)
		assert.Equal(t, execution.PID(12345), result.PID)
		assert.Equal(t, int64(1234567890), result.CreatedAt)
		assert.Equal(t, "2024-01-01T00:00:00Z", result.QueuedAt)
		assert.Equal(t, "2024-01-01T00:01:00Z", result.StartedAt)
		assert.Equal(t, "2024-01-01T00:02:00Z", result.FinishedAt)
		assert.Equal(t, "/path/to/log", result.Log)
		assert.Equal(t, "test error", result.Error)
		assert.Equal(t, "key=value", result.Params)
		assert.Equal(t, []string{"key=value"}, result.ParamsList)
	})

	t.Run("with root and parent refs", func(t *testing.T) {
		proto := &coordinatorv1.DAGRunStatusProto{
			Name:     "test-dag",
			DagRunId: "run-123",
			Root:     &coordinatorv1.DAGRunRefProto{Name: "root-dag", Id: "root-run"},
			Parent:   &coordinatorv1.DAGRunRefProto{Name: "parent-dag", Id: "parent-run"},
		}

		result := ProtoToDAGRunStatus(proto)
		require.NotNil(t, result)
		assert.Equal(t, "root-dag", result.Root.Name)
		assert.Equal(t, "root-run", result.Root.ID)
		assert.Equal(t, "parent-dag", result.Parent.Name)
		assert.Equal(t, "parent-run", result.Parent.ID)
	})
}

func TestProtoToDAGRunRef(t *testing.T) {
	t.Run("nil proto", func(t *testing.T) {
		result := ProtoToDAGRunRef(nil)
		assert.True(t, result.Zero())
	})

	t.Run("valid proto", func(t *testing.T) {
		proto := &coordinatorv1.DAGRunRefProto{Name: "test-dag", Id: "run-123"}
		result := ProtoToDAGRunRef(proto)
		assert.Equal(t, "test-dag", result.Name)
		assert.Equal(t, "run-123", result.ID)
	})
}

func TestProtoToNode(t *testing.T) {
	t.Run("nil proto", func(t *testing.T) {
		result := ProtoToNode(nil)
		assert.Nil(t, result)
	})

	t.Run("basic fields", func(t *testing.T) {
		proto := &coordinatorv1.NodeStatusProto{
			StepName:   "test-step",
			Status:     int32(core.NodeSucceeded),
			Stdout:     "/path/stdout.log",
			Stderr:     "/path/stderr.log",
			StartedAt:  "2024-01-01T00:00:00Z",
			FinishedAt: "2024-01-01T00:01:00Z",
			Error:      "test error",
			RetryCount: 2,
			DoneCount:  3,
			RetriedAt:  "2024-01-01T00:00:30Z",
		}

		result := ProtoToNode(proto)
		require.NotNil(t, result)
		assert.Equal(t, "test-step", result.Step.Name)
		assert.Equal(t, core.NodeSucceeded, result.Status)
		assert.Equal(t, "/path/stdout.log", result.Stdout)
		assert.Equal(t, "/path/stderr.log", result.Stderr)
		assert.Equal(t, "2024-01-01T00:00:00Z", result.StartedAt)
		assert.Equal(t, "2024-01-01T00:01:00Z", result.FinishedAt)
		assert.Equal(t, "test error", result.Error)
		assert.Equal(t, 2, result.RetryCount)
		assert.Equal(t, 3, result.DoneCount)
		assert.Equal(t, "2024-01-01T00:00:30Z", result.RetriedAt)
	})

	t.Run("with step info and executor type", func(t *testing.T) {
		proto := &coordinatorv1.NodeStatusProto{
			StepName: "test-step",
			Step: &coordinatorv1.StepProto{
				Name:         "test-step",
				Description:  "test description",
				ExecutorType: "docker",
			},
		}

		result := ProtoToNode(proto)
		require.NotNil(t, result)
		assert.Equal(t, "test-step", result.Step.Name)
		assert.Equal(t, "test description", result.Step.Description)
		assert.Equal(t, "docker", result.Step.ExecutorConfig.Type)
	})

	t.Run("with sub runs", func(t *testing.T) {
		proto := &coordinatorv1.NodeStatusProto{
			StepName: "test-step",
			SubRuns: []*coordinatorv1.SubDAGRunProto{
				{DagRunId: "sub-run-1", Params: "p1=v1"},
				{DagRunId: "sub-run-2", Params: "p2=v2"},
			},
		}

		result := ProtoToNode(proto)
		require.NotNil(t, result)
		require.Len(t, result.SubRuns, 2)
		assert.Equal(t, "sub-run-1", result.SubRuns[0].DAGRunID)
		assert.Equal(t, "p1=v1", result.SubRuns[0].Params)
		assert.Equal(t, "sub-run-2", result.SubRuns[1].DAGRunID)
		assert.Equal(t, "p2=v2", result.SubRuns[1].Params)
	})

	t.Run("with output variables", func(t *testing.T) {
		proto := &coordinatorv1.NodeStatusProto{
			StepName: "test-step",
			OutputVariables: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		}

		result := ProtoToNode(proto)
		require.NotNil(t, result)
		require.NotNil(t, result.OutputVariables)

		var val1, val2 string
		result.OutputVariables.Range(func(key, value any) bool {
			if key == "key1" {
				val1 = value.(string)
			}
			if key == "key2" {
				val2 = value.(string)
			}
			return true
		})
		assert.Equal(t, "value1", val1)
		assert.Equal(t, "value2", val2)
	})
}

func TestRoundTrip(t *testing.T) {
	t.Run("status roundtrip", func(t *testing.T) {
		original := &execution.DAGRunStatus{
			Name:       "test-dag",
			DAGRunID:   "run-123",
			AttemptID:  "attempt-1",
			Status:     core.Running,
			WorkerID:   "worker-1",
			PID:        12345,
			Root:       execution.DAGRunRef{Name: "root-dag", ID: "root-run"},
			Parent:     execution.DAGRunRef{Name: "parent-dag", ID: "parent-run"},
			CreatedAt:  1234567890,
			QueuedAt:   "2024-01-01T00:00:00Z",
			StartedAt:  "2024-01-01T00:01:00Z",
			FinishedAt: "2024-01-01T00:02:00Z",
			Log:        "/path/to/log",
			Error:      "test error",
			Params:     "key=value",
			ParamsList: []string{"key=value"},
			Nodes: []*execution.Node{
				{
					Step: core.Step{
						Name:           "step-1",
						Description:    "first step",
						ExecutorConfig: core.ExecutorConfig{Type: "shell"},
					},
					Status:     core.NodeSucceeded,
					StartedAt:  "2024-01-01T00:00:00Z",
					FinishedAt: "2024-01-01T00:01:00Z",
				},
			},
		}

		// Convert to proto and back
		proto := DAGRunStatusToProto(original)
		result := ProtoToDAGRunStatus(proto)

		// Verify key fields are preserved
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
		require.Len(t, result.Nodes, 1)
		assert.Equal(t, original.Nodes[0].Step.Name, result.Nodes[0].Step.Name)
		assert.Equal(t, original.Nodes[0].Step.Description, result.Nodes[0].Step.Description)
		assert.Equal(t, original.Nodes[0].Step.ExecutorConfig.Type, result.Nodes[0].Step.ExecutorConfig.Type)
	})
}
