package models_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/google/uuid"

	"github.com/stretchr/testify/require"
)

func TestStatusSerialization(t *testing.T) {
	startedAt, finishedAt := time.Now(), time.Now().Add(time.Second*1)
	dag := &digraph.DAG{
		HandlerOn: digraph.HandlerOn{},
		Steps: []digraph.Step{
			{
				Name: "1", Description: "",
				Dir: "dir", Command: "echo 1", Args: []string{},
				Depends: []string{}, ContinueOn: digraph.ContinueOn{},
				RetryPolicy: digraph.RetryPolicy{}, MailOnError: false,
				RepeatPolicy: digraph.RepeatPolicy{}, Preconditions: []*digraph.Condition{},
			},
		},
		MailOn:    &digraph.MailOn{},
		ErrorMail: &digraph.MailConfig{},
		InfoMail:  &digraph.MailConfig{},
		SMTP:      &digraph.SMTPConfig{},
	}
	dagRunID := uuid.Must(uuid.NewV7()).String()
	statusToPersist := models.NewStatusBuilder(dag).Create(dagRunID, scheduler.StatusSuccess, 0, startedAt, models.WithFinishedAt(finishedAt))

	rawJSON, err := json.Marshal(statusToPersist)
	require.NoError(t, err)

	statusObject, err := models.StatusFromJSON(string(rawJSON))
	require.NoError(t, err)

	require.Equal(t, statusToPersist.Name, statusObject.Name)
	require.Equal(t, 1, len(statusObject.Nodes))
	require.Equal(t, dag.Steps[0].Name, statusObject.Nodes[0].Step.Name)
}

func TestDAGRunStatus_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		status   models.DAGRunStatus
		validate func(t *testing.T, data []byte)
	}{
		{
			name: "marshal with log directory placeholder replacement",
			status: models.DAGRunStatus{
				Name:     "test-dag",
				DAGRunID: "test-run-123",
				Status:   scheduler.StatusRunning,
				Log:      "/var/log/dagu/test-dag/test-run-123.log",
				Nodes: []*models.Node{
					{
						Step:   digraph.Step{Name: "step1"},
						Stdout: "/var/log/dagu/test-dag/step1.stdout.log",
						Stderr: "/var/log/dagu/test-dag/step1.stderr.log",
						Status: scheduler.NodeStatusRunning,
					},
					{
						Step:   digraph.Step{Name: "step2"},
						Stdout: "/var/log/dagu/test-dag/step2.stdout.log",
						Stderr: "/var/log/dagu/test-dag/step2.stderr.log",
						Status: scheduler.NodeStatusCancel,
					},
				},
			},
			validate: func(t *testing.T, data []byte) {
				var result map[string]any
				err := json.Unmarshal(data, &result)
				require.NoError(t, err)

				// Check that log directory was replaced with placeholder
				nodes := result["nodes"].([]any)
				require.Len(t, nodes, 2)

				node1 := nodes[0].(map[string]any)
				require.Equal(t, "__INTERNAL_LOG_DIR__/step1.stdout.log", node1["stdout"])
				require.Equal(t, "__INTERNAL_LOG_DIR__/step1.stderr.log", node1["stderr"])

				node2 := nodes[1].(map[string]any)
				require.Equal(t, "__INTERNAL_LOG_DIR__/step2.stdout.log", node2["stdout"])
				require.Equal(t, "__INTERNAL_LOG_DIR__/step2.stderr.log", node2["stderr"])
			},
		},
		{
			name: "marshal without log (no replacement)",
			status: models.DAGRunStatus{
				Name:     "test-dag",
				DAGRunID: "test-run-456",
				Status:   scheduler.StatusSuccess,
				Nodes: []*models.Node{
					{
						Step:   digraph.Step{Name: "step1"},
						Stdout: "/var/log/dagu/test-dag/step1.stdout.log",
						Stderr: "/some/path/step1.stderr.log",
						Status: scheduler.NodeStatusSuccess,
					},
				},
			},
			validate: func(t *testing.T, data []byte) {
				var result map[string]any
				err := json.Unmarshal(data, &result)
				require.NoError(t, err)

				// Without log path, the paths should remain unchanged
				nodes := result["nodes"].([]any)
				require.Len(t, nodes, 1)

				node := nodes[0].(map[string]any)
				require.Equal(t, "/var/log/dagu/test-dag/step1.stdout.log", node["stdout"])
				require.Equal(t, "/some/path/step1.stderr.log", node["stderr"])
			},
		},
		{
			name: "marshal with nil nodes",
			status: models.DAGRunStatus{
				Name:     "test-dag",
				DAGRunID: "test-run-789",
				Status:   scheduler.StatusSuccess,
				Log:      "/var/log/dagu/test-dag/test-run-789.log",
				Nodes: []*models.Node{
					{
						Step:   digraph.Step{Name: "step1"},
						Stdout: "/var/log/dagu/test-dag/step1.stdout.log",
						Status: scheduler.NodeStatusSuccess,
					},
					nil, // nil node should be handled gracefully
					{
						Step:   digraph.Step{Name: "step3"},
						Stderr: "/var/log/dagu/test-dag/step3.stderr.log",
						Status: scheduler.NodeStatusError,
					},
				},
			},
			validate: func(t *testing.T, data []byte) {
				var result map[string]any
				err := json.Unmarshal(data, &result)
				require.NoError(t, err)

				nodes := result["nodes"].([]any)
				require.Len(t, nodes, 3)

				// First node
				node1 := nodes[0].(map[string]any)
				require.Equal(t, "__INTERNAL_LOG_DIR__/step1.stdout.log", node1["stdout"])

				// Second node should be nil
				require.Nil(t, nodes[1])

				// Third node
				node3 := nodes[2].(map[string]any)
				require.Equal(t, "__INTERNAL_LOG_DIR__/step3.stderr.log", node3["stderr"])
			},
		},
		{
			name: "marshal with handler nodes",
			status: models.DAGRunStatus{
				Name:     "test-dag",
				DAGRunID: "test-run-handler",
				Status:   scheduler.StatusSuccess,
				Log:      "/var/log/dagu/test-dag/test-run-handler.log",
				OnExit: &models.Node{
					Step:   digraph.Step{Name: "onExit"},
					Stdout: "/var/log/dagu/test-dag/onExit.stdout.log",
					Status: scheduler.NodeStatusSuccess,
				},
				OnFailure: &models.Node{
					Step:   digraph.Step{Name: "onFailure"},
					Stdout: "/var/log/dagu/test-dag/onFailure.stdout.log",
					Status: scheduler.NodeStatusSkipped,
				},
			},
			validate: func(t *testing.T, data []byte) {
				var result map[string]any
				err := json.Unmarshal(data, &result)
				require.NoError(t, err)

				// Handler nodes should also have their paths replaced with placeholders
				onExit := result["onExit"].(map[string]any)
				require.Equal(t, "__INTERNAL_LOG_DIR__/onExit.stdout.log", onExit["stdout"])

				onFailure := result["onFailure"].(map[string]any)
				require.Equal(t, "__INTERNAL_LOG_DIR__/onFailure.stdout.log", onFailure["stdout"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.status.MarshalJSON()
			require.NoError(t, err)
			require.NotNil(t, data)

			tt.validate(t, data)
		})
	}
}

func TestStatusFromJSON(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		wantErr  bool
		validate func(t *testing.T, status *models.DAGRunStatus)
	}{
		{
			name: "deserialize with log directory placeholder replacement",
			jsonData: `{
				"name": "test-dag",
				"dagRunId": "test-run-123",
				"status": 1,
				"log": "/var/log/dagu/test-dag/test-run-123.log",
				"nodes": [
					{
						"step": {"name": "step1"},
						"stdout": "__INTERNAL_LOG_DIR__/step1.stdout.log",
						"stderr": "__INTERNAL_LOG_DIR__/step1.stderr.log",
						"status": 4
					},
					{
						"step": {"name": "step2"},
						"stdout": "__INTERNAL_LOG_DIR__/step2.stdout.log",
						"stderr": "__INTERNAL_LOG_DIR__/step2.stderr.log",
						"status": 2
					}
				]
			}`,
			validate: func(t *testing.T, status *models.DAGRunStatus) {
				require.Equal(t, "test-dag", status.Name)
				require.Equal(t, "test-run-123", status.DAGRunID)
				require.Equal(t, scheduler.StatusRunning, status.Status)
				require.Equal(t, "/var/log/dagu/test-dag/test-run-123.log", status.Log)

				require.Len(t, status.Nodes, 2)

				// Check that log directory placeholders were replaced
				require.Equal(t, "/var/log/dagu/test-dag/step1.stdout.log", status.Nodes[0].Stdout)
				require.Equal(t, "/var/log/dagu/test-dag/step1.stderr.log", status.Nodes[0].Stderr)

				require.Equal(t, "/var/log/dagu/test-dag/step2.stdout.log", status.Nodes[1].Stdout)
				require.Equal(t, "/var/log/dagu/test-dag/step2.stderr.log", status.Nodes[1].Stderr)
			},
		},
		{
			name: "deserialize without log (no replacement)",
			jsonData: `{
				"name": "test-dag",
				"dagRunId": "test-run-456",
				"status": 4,
				"nodes": [
					{
						"step": {"name": "step1"},
						"stdout": "__INTERNAL_LOG_DIR__/step1.stdout.log",
						"stderr": "__INTERNAL_LOG_DIR__/step1.stderr.log",
						"status": 4
					}
				]
			}`,
			validate: func(t *testing.T, status *models.DAGRunStatus) {
				require.Equal(t, "test-dag", status.Name)
				require.Equal(t, "", status.Log)

				// Placeholders should remain unchanged when no log path
				require.Equal(t, "__INTERNAL_LOG_DIR__/step1.stdout.log", status.Nodes[0].Stdout)
				require.Equal(t, "__INTERNAL_LOG_DIR__/step1.stderr.log", status.Nodes[0].Stderr)
			},
		},
		{
			name: "deserialize with nil nodes",
			jsonData: `{
				"name": "test-dag",
				"dagRunId": "test-run-789",
				"status": 4,
				"log": "/var/log/dagu/test-dag/test-run-789.log",
				"nodes": [
					{
						"step": {"name": "step1"},
						"stdout": "__INTERNAL_LOG_DIR__/step1.stdout.log",
						"status": 4
					},
					null,
					{
						"step": {"name": "step3"},
						"stderr": "__INTERNAL_LOG_DIR__/step3.stderr.log",
						"status": 2
					}
				]
			}`,
			validate: func(t *testing.T, status *models.DAGRunStatus) {
				require.Len(t, status.Nodes, 3)

				require.NotNil(t, status.Nodes[0])
				require.Equal(t, "/var/log/dagu/test-dag/step1.stdout.log", status.Nodes[0].Stdout)

				require.Nil(t, status.Nodes[1])

				require.NotNil(t, status.Nodes[2])
				require.Equal(t, "/var/log/dagu/test-dag/step3.stderr.log", status.Nodes[2].Stderr)
			},
		},
		{
			name: "deserialize with log path ending with slash",
			jsonData: `{
				"name": "test-dag",
				"dagRunId": "test-run-slash",
				"status": 4,
				"log": "/var/log/dagu/test-dag//test-run-slash.log",
				"nodes": [
					{
						"step": {"name": "step1"},
						"stdout": "__INTERNAL_LOG_DIR__/step1.stdout.log",
						"stderr": "__INTERNAL_LOG_DIR__/step1.stderr.log",
						"status": 4
					}
				]
			}`,
			validate: func(t *testing.T, status *models.DAGRunStatus) {
				// Should handle trailing slash correctly
				require.Equal(t, "/var/log/dagu/test-dag/step1.stdout.log", status.Nodes[0].Stdout)
				require.Equal(t, "/var/log/dagu/test-dag/step1.stderr.log", status.Nodes[0].Stderr)
			},
		},
		{
			name: "deserialize with complex hierarchy",
			jsonData: `{
				"name": "parent-dag",
				"dagRunId": "parent-run-123",
				"status": 1,
				"root": {"name": "root-dag", "id": "root-123"},
				"parent": {"name": "grandparent-dag", "id": "grandparent-123"},
				"createdAt": 1234567890,
				"startedAt": "2024-01-01T10:00:00",
				"finishedAt": "2024-01-01T11:00:00",
				"params": "param1 param2",
				"paramsList": ["param1", "param2"]
			}`,
			validate: func(t *testing.T, status *models.DAGRunStatus) {
				require.Equal(t, "parent-dag", status.Name)
				require.Equal(t, "parent-run-123", status.DAGRunID)
				require.Equal(t, "root-dag", status.Root.Name)
				require.Equal(t, "root-123", status.Root.ID)
				require.Equal(t, "grandparent-dag", status.Parent.Name)
				require.Equal(t, "grandparent-123", status.Parent.ID)
				require.Equal(t, int64(1234567890), status.CreatedAt)
				require.Equal(t, "2024-01-01T10:00:00", status.StartedAt)
				require.Equal(t, "2024-01-01T11:00:00", status.FinishedAt)
				require.Equal(t, "param1 param2", status.Params)
				require.Equal(t, []string{"param1", "param2"}, status.ParamsList)
			},
		},
		{
			name:     "invalid JSON",
			jsonData: `{invalid json`,
			wantErr:  true,
		},
		{
			name:     "empty JSON",
			jsonData: ``,
			wantErr:  true,
		},
		{
			name: "deserialize with handler nodes",
			jsonData: `{
				"name": "test-dag",
				"dagRunId": "test-run-handlers",
				"status": 4,
				"log": "/var/log/dagu/test-dag/test-run-handlers.log",
				"onExit": {
					"step": {"name": "cleanup"},
					"stdout": "__INTERNAL_LOG_DIR__/cleanup.stdout.log",
					"stderr": "__INTERNAL_LOG_DIR__/cleanup.stderr.log",
					"status": 4
				},
				"onFailure": {
					"step": {"name": "notify"},
					"stdout": "__INTERNAL_LOG_DIR__/notify.stdout.log",
					"status": 5
				}
			}`,
			validate: func(t *testing.T, status *models.DAGRunStatus) {
				require.Equal(t, "test-dag", status.Name)
				require.Equal(t, "/var/log/dagu/test-dag/test-run-handlers.log", status.Log)
				
				// Check handler nodes have placeholders replaced
				require.NotNil(t, status.OnExit)
				require.Equal(t, "/var/log/dagu/test-dag/cleanup.stdout.log", status.OnExit.Stdout)
				require.Equal(t, "/var/log/dagu/test-dag/cleanup.stderr.log", status.OnExit.Stderr)
				
				require.NotNil(t, status.OnFailure)
				require.Equal(t, "/var/log/dagu/test-dag/notify.stdout.log", status.OnFailure.Stdout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := models.StatusFromJSON(tt.jsonData)

			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, status)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, status)
			tt.validate(t, status)
		})
	}
}


func TestDAGRunStatus_MarshalJSON_DoesNotModifyOriginal(t *testing.T) {
	// Test that MarshalJSON does not modify the original data
	original := models.DAGRunStatus{
		Name:     "test-dag",
		DAGRunID: "test-run-immutable",
		Status:   scheduler.StatusRunning,
		Log:      "/var/log/dagu/test-dag/test-run-immutable.log",
		Nodes: []*models.Node{
			{
				Step:   digraph.Step{Name: "step1"},
				Stdout: "/var/log/dagu/test-dag/step1.stdout.log",
				Stderr: "/var/log/dagu/test-dag/step1.stderr.log",
				Status: scheduler.NodeStatusRunning,
			},
		},
		OnExit: &models.Node{
			Step:   digraph.Step{Name: "onExit"},
			Stdout: "/var/log/dagu/test-dag/onExit.stdout.log",
			Status: scheduler.NodeStatusNone,
		},
	}

	// Store original values
	originalNodeStdout := original.Nodes[0].Stdout
	originalNodeStderr := original.Nodes[0].Stderr
	originalOnExitStdout := original.OnExit.Stdout

	// Marshal (this should not modify the original)
	data, err := original.MarshalJSON()
	require.NoError(t, err)

	// Verify original data is unchanged
	require.Equal(t, "/var/log/dagu/test-dag/step1.stdout.log", original.Nodes[0].Stdout)
	require.Equal(t, "/var/log/dagu/test-dag/step1.stderr.log", original.Nodes[0].Stderr)
	require.Equal(t, "/var/log/dagu/test-dag/onExit.stdout.log", original.OnExit.Stdout)
	require.Equal(t, originalNodeStdout, original.Nodes[0].Stdout)
	require.Equal(t, originalNodeStderr, original.Nodes[0].Stderr)
	require.Equal(t, originalOnExitStdout, original.OnExit.Stdout)

	// Verify marshaled data has placeholders
	var result map[string]any
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	nodes := result["nodes"].([]any)
	node := nodes[0].(map[string]any)
	require.Equal(t, "__INTERNAL_LOG_DIR__/step1.stdout.log", node["stdout"])
	require.Equal(t, "__INTERNAL_LOG_DIR__/step1.stderr.log", node["stderr"])

	onExit := result["onExit"].(map[string]any)
	require.Equal(t, "__INTERNAL_LOG_DIR__/onExit.stdout.log", onExit["stdout"])
}

func TestDAGRunStatus_MarshalJSON_RoundTrip(t *testing.T) {
	// Test that marshaling and unmarshaling preserves data correctly
	original := models.DAGRunStatus{
		Name:       "test-dag",
		DAGRunID:   "test-run-roundtrip",
		AttemptID:  "attempt-1",
		Status:     scheduler.StatusRunning,
		PID:        models.PID(12345),
		CreatedAt:  time.Now().UnixMilli(),
		QueuedAt:   "2024-01-01T09:00:00",
		StartedAt:  "2024-01-01T10:00:00",
		FinishedAt: "2024-01-01T11:00:00",
		Log:        "/var/log/dagu/test-dag/test-run-roundtrip.log",
		Params:     "param1=value1 param2=value2",
		ParamsList: []string{"param1=value1", "param2=value2"},
		Nodes: []*models.Node{
			{
				Step:       digraph.Step{Name: "step1", Command: "echo hello"},
				Stdout:     "/var/log/dagu/test-dag/step1.stdout.log",
				Stderr:     "/var/log/dagu/test-dag/step1.stderr.log",
				StartedAt:  "2024-01-01T10:00:00",
				FinishedAt: "2024-01-01T10:05:00",
				Status:     scheduler.NodeStatusSuccess,
				RetryCount: 2,
				DoneCount:  1,
			},
			{
				Step:       digraph.Step{Name: "step2", Command: "echo world"},
				Stdout:     "/var/log/dagu/test-dag/step2.stdout.log",
				Stderr:     "/var/log/dagu/test-dag/step2.stderr.log",
				StartedAt:  "2024-01-01T10:05:00",
				FinishedAt: "2024-01-01T10:10:00",
				Status:     scheduler.NodeStatusError,
				Error:      "command failed",
			},
		},
		OnExit: &models.Node{
			Step:   digraph.Step{Name: "cleanup", Command: "rm -f /tmp/data"},
			Status: scheduler.NodeStatusCancel,
		},
		Preconditions: []*digraph.Condition{
			{
				Condition: "test -f /tmp/input.txt",
				Expected:  "true",
			},
		},
	}

	// Marshal
	jsonData, err := json.Marshal(original)
	require.NoError(t, err)

	// Unmarshal
	result, err := models.StatusFromJSON(string(jsonData))
	require.NoError(t, err)

	// Verify main fields
	require.Equal(t, original.Name, result.Name)
	require.Equal(t, original.DAGRunID, result.DAGRunID)
	require.Equal(t, original.AttemptID, result.AttemptID)
	require.Equal(t, original.Status, result.Status)
	require.Equal(t, original.PID, result.PID)
	require.Equal(t, original.CreatedAt, result.CreatedAt)
	require.Equal(t, original.QueuedAt, result.QueuedAt)
	require.Equal(t, original.StartedAt, result.StartedAt)
	require.Equal(t, original.FinishedAt, result.FinishedAt)
	require.Equal(t, original.Log, result.Log)
	require.Equal(t, original.Params, result.Params)
	require.Equal(t, original.ParamsList, result.ParamsList)

	// Verify nodes with placeholder replacement
	require.Len(t, result.Nodes, 2)
	require.Equal(t, "/var/log/dagu/test-dag/step1.stdout.log", result.Nodes[0].Stdout)
	require.Equal(t, "/var/log/dagu/test-dag/step1.stderr.log", result.Nodes[0].Stderr)
	require.Equal(t, "/var/log/dagu/test-dag/step2.stdout.log", result.Nodes[1].Stdout)
	require.Equal(t, "/var/log/dagu/test-dag/step2.stderr.log", result.Nodes[1].Stderr)

	// Verify other node fields
	require.Equal(t, original.Nodes[0].Status, result.Nodes[0].Status)
	require.Equal(t, original.Nodes[0].RetryCount, result.Nodes[0].RetryCount)
	require.Equal(t, original.Nodes[1].Error, result.Nodes[1].Error)

	// Verify handler nodes
	require.NotNil(t, result.OnExit)
	require.Equal(t, original.OnExit.Step.Name, result.OnExit.Step.Name)

	// Verify preconditions
	require.Len(t, result.Preconditions, 1)
	require.Equal(t, original.Preconditions[0].Condition, result.Preconditions[0].Condition)
}
