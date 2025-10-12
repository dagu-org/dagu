package model_test

import (
	"errors"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/common/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/status"
	"github.com/dagu-org/dagu/internal/persistence/legacy/model"
	"github.com/dagu-org/dagu/internal/runtime/scheduler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromSteps(t *testing.T) {
	steps := []core.Step{
		{
			Name:    "step1",
			Command: "echo hello",
		},
		{
			Name:    "step2",
			Command: "echo world",
			Depends: []string{"step1"},
		},
	}

	nodes := model.FromSteps(steps)

	assert.Len(t, nodes, 2)
	assert.Equal(t, "step1", nodes[0].Step.Name)
	assert.Equal(t, "step2", nodes[1].Step.Name)
	assert.Equal(t, "-", nodes[0].StartedAt)
	assert.Equal(t, "-", nodes[0].FinishedAt)
	assert.Equal(t, status.NodeNone, nodes[0].Status)
	assert.Equal(t, status.NodeNone.String(), nodes[0].StatusText)
}

func TestFromNodes(t *testing.T) {
	now := time.Now()
	later := now.Add(5 * time.Second)
	retryTime := now.Add(2 * time.Second)

	nodeDataList := []scheduler.NodeData{
		{
			Step: core.Step{
				Name:    "step1",
				Command: "echo hello",
			},
			State: scheduler.NodeState{
				Status:     status.NodeSuccess,
				Stdout:     "/tmp/step1.log",
				StartedAt:  now,
				FinishedAt: later,
				RetryCount: 2,
				RetriedAt:  retryTime,
				DoneCount:  1,
			},
		},
		{
			Step: core.Step{
				Name:    "step2",
				Command: "false",
			},
			State: scheduler.NodeState{
				Status:     status.NodeError,
				Stdout:     "/tmp/step2.log",
				StartedAt:  now,
				FinishedAt: later,
				Error:      errors.New("command failed"),
			},
		},
	}

	nodes := model.FromNodes(nodeDataList)

	assert.Len(t, nodes, 2)

	// Check first node
	assert.Equal(t, "step1", nodes[0].Step.Name)
	assert.Equal(t, "/tmp/step1.log", nodes[0].Log)
	assert.Equal(t, status.NodeSuccess, nodes[0].Status)
	assert.Equal(t, status.NodeSuccess.String(), nodes[0].StatusText)
	assert.Equal(t, stringutil.FormatTime(now), nodes[0].StartedAt)
	assert.Equal(t, stringutil.FormatTime(later), nodes[0].FinishedAt)
	assert.Equal(t, stringutil.FormatTime(retryTime), nodes[0].RetriedAt)
	assert.Equal(t, 2, nodes[0].RetryCount)
	assert.Equal(t, 1, nodes[0].DoneCount)
	assert.Empty(t, nodes[0].Error)

	// Check second node
	assert.Equal(t, "step2", nodes[1].Step.Name)
	assert.Equal(t, status.NodeError, nodes[1].Status)
	assert.Equal(t, "command failed", nodes[1].Error)
}

func TestFromNode(t *testing.T) {
	now := time.Now()
	later := now.Add(5 * time.Second)

	nodeData := scheduler.NodeData{
		Step: core.Step{
			Name:        "test-step",
			Command:     "echo test",
			Description: "Test step",
			Dir:         "/tmp",
		},
		State: scheduler.NodeState{
			Status:     status.NodeSuccess,
			Stdout:     "/tmp/test.log",
			StartedAt:  now,
			FinishedAt: later,
			RetryCount: 1,
			DoneCount:  2,
			Error:      nil,
		},
	}

	node := model.FromNode(nodeData)

	assert.Equal(t, "test-step", node.Step.Name)
	assert.Equal(t, "echo test", node.Step.Command)
	assert.Equal(t, "/tmp/test.log", node.Log)
	assert.Equal(t, status.NodeSuccess, node.Status)
	assert.Equal(t, status.NodeSuccess.String(), node.StatusText)
	assert.Equal(t, stringutil.FormatTime(now), node.StartedAt)
	assert.Equal(t, stringutil.FormatTime(later), node.FinishedAt)
	assert.Equal(t, 1, node.RetryCount)
	assert.Equal(t, 2, node.DoneCount)
	assert.Empty(t, node.Error)
}

func TestNewNode(t *testing.T) {
	step := core.Step{
		Name:        "new-step",
		Command:     "ls -la",
		Description: "List files",
		Args:        []string{"-la"},
		Dir:         "/home",
	}

	node := model.NewNode(step)

	assert.Equal(t, step, node.Step)
	assert.Equal(t, "-", node.StartedAt)
	assert.Equal(t, "-", node.FinishedAt)
	assert.Equal(t, status.NodeNone, node.Status)
	assert.Equal(t, status.NodeNone.String(), node.StatusText)
	assert.Empty(t, node.Log)
	assert.Empty(t, node.Error)
	assert.Empty(t, node.RetriedAt)
	assert.Equal(t, 0, node.RetryCount)
	assert.Equal(t, 0, node.DoneCount)
}

func TestNodeToNode(t *testing.T) {
	now := time.Now()
	later := now.Add(10 * time.Second)
	retryTime := now.Add(5 * time.Second)

	modelNode := &model.Node{
		Step: core.Step{
			Name:    "convert-step",
			Command: "sleep 1",
		},
		Log:        "/var/log/step.log",
		StartedAt:  stringutil.FormatTime(now),
		FinishedAt: stringutil.FormatTime(later),
		Status:     status.NodeSuccess,
		StatusText: status.NodeSuccess.String(),
		RetriedAt:  stringutil.FormatTime(retryTime),
		RetryCount: 3,
		DoneCount:  4,
		Error:      "some error occurred",
	}

	schedulerNode := modelNode.ToNode()

	assert.Equal(t, modelNode.Step, schedulerNode.NodeData().Step)
	assert.Equal(t, modelNode.Log, schedulerNode.NodeData().State.Stdout)
	assert.Equal(t, modelNode.Status, schedulerNode.NodeData().State.Status)
	assert.Equal(t, modelNode.RetryCount, schedulerNode.NodeData().State.RetryCount)
	assert.Equal(t, modelNode.DoneCount, schedulerNode.NodeData().State.DoneCount)

	// Check times
	expectedStart, _ := stringutil.ParseTime(modelNode.StartedAt)
	expectedFinish, _ := stringutil.ParseTime(modelNode.FinishedAt)
	expectedRetry, _ := stringutil.ParseTime(modelNode.RetriedAt)

	assert.Equal(t, expectedStart, schedulerNode.NodeData().State.StartedAt)
	assert.Equal(t, expectedFinish, schedulerNode.NodeData().State.FinishedAt)
	assert.Equal(t, expectedRetry, schedulerNode.NodeData().State.RetriedAt)

	// Check error
	if modelNode.Error != "" {
		assert.NotNil(t, schedulerNode.NodeData().State.Error)
		assert.Contains(t, schedulerNode.NodeData().State.Error.Error(), "some error occurred")
	} else {
		assert.Nil(t, schedulerNode.NodeData().State.Error)
	}
}

func TestNodeToNodeWithEmptyTimes(t *testing.T) {
	modelNode := &model.Node{
		Step: core.Step{
			Name: "empty-times-step",
		},
		StartedAt:  "-",
		FinishedAt: "-",
		RetriedAt:  "",
		Status:     status.NodeNone,
		StatusText: status.NodeNone.String(),
	}

	schedulerNode := modelNode.ToNode()

	assert.True(t, schedulerNode.NodeData().State.StartedAt.IsZero())
	assert.True(t, schedulerNode.NodeData().State.FinishedAt.IsZero())
	assert.True(t, schedulerNode.NodeData().State.RetriedAt.IsZero())
}

func TestNodeToNodeWithInvalidTimeFormat(t *testing.T) {
	modelNode := &model.Node{
		Step: core.Step{
			Name: "invalid-time-step",
		},
		StartedAt:  "invalid-time-format",
		FinishedAt: "2024-13-45 25:61:70", // Invalid date/time
		Status:     status.NodeError,
		StatusText: status.NodeError.String(),
	}

	schedulerNode := modelNode.ToNode()

	// ParseTime should return zero time for invalid formats
	assert.True(t, schedulerNode.NodeData().State.StartedAt.IsZero())
	assert.True(t, schedulerNode.NodeData().State.FinishedAt.IsZero())
}

func TestErrFromText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected error
	}{
		{
			name:     "EmptyString",
			input:    "",
			expected: nil,
		},
		{
			name:     "ErrorMessage",
			input:    "command failed with exit code 1",
			expected: errors.New("node processing error: command failed with exit code 1"),
		},
		{
			name:     "WhitespaceOnly",
			input:    "   ",
			expected: errors.New("node processing error:    "),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We need to test through the public interface
			node := &model.Node{Error: tt.input}
			schedulerNode := node.ToNode()

			if tt.expected == nil {
				assert.Nil(t, schedulerNode.NodeData().State.Error)
			} else {
				require.NotNil(t, schedulerNode.NodeData().State.Error)
				assert.Equal(t, tt.expected.Error(), schedulerNode.NodeData().State.Error.Error())
			}
		})
	}
}

func TestErrText(t *testing.T) {
	tests := []struct {
		name     string
		input    error
		expected string
	}{
		{
			name:     "NilError",
			input:    nil,
			expected: "",
		},
		{
			name:     "SimpleError",
			input:    errors.New("test error"),
			expected: "test error",
		},
		{
			name:     "WrappedError",
			input:    errors.New("wrapped: original error"),
			expected: "wrapped: original error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test through the public interface
			nodeData := scheduler.NodeData{
				Step: core.Step{Name: "test"},
				State: scheduler.NodeState{
					Error: tt.input,
				},
			}

			node := model.FromNode(nodeData)
			assert.Equal(t, tt.expected, node.Error)
		})
	}
}

func TestNodeWithAllStatuses(t *testing.T) {
	statuses := []status.NodeStatus{
		status.NodeNone,
		status.NodeRunning,
		status.NodeError,
		status.NodeCancel,
		status.NodeSuccess,
		status.NodeSkipped,
	}

	for _, status := range statuses {
		t.Run(status.String(), func(t *testing.T) {
			nodeData := scheduler.NodeData{
				Step: core.Step{
					Name: "status-test-step",
				},
				State: scheduler.NodeState{
					Status: status,
				},
			}

			modelNode := model.FromNode(nodeData)
			assert.Equal(t, status, modelNode.Status)
			assert.Equal(t, status.String(), modelNode.StatusText)

			// Convert back
			schedulerNode := modelNode.ToNode()
			assert.Equal(t, status, schedulerNode.NodeData().State.Status)
		})
	}
}

func TestFromNodesPreservesOrder(t *testing.T) {
	var nodeDataList []scheduler.NodeData
	for i := 0; i < 10; i++ {
		nodeDataList = append(nodeDataList, scheduler.NodeData{
			Step: core.Step{
				Name: string(rune('A' + i)),
			},
			State: scheduler.NodeState{
				Status: status.NodeSuccess,
			},
		})
	}

	nodes := model.FromNodes(nodeDataList)

	assert.Len(t, nodes, 10)
	for i := 0; i < 10; i++ {
		assert.Equal(t, string(rune('A'+i)), nodes[i].Step.Name)
	}
}
