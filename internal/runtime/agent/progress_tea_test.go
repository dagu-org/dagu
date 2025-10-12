package agent

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/status"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgressModel_Init(t *testing.T) {
	dag := &core.DAG{
		Name: "test-dag",
		Steps: []core.Step{
			{Name: "step1"},
			{Name: "step2"},
		},
	}

	model := NewProgressModel(dag)

	// Test initialization
	assert.Equal(t, "test-dag", model.dag.Name)
	assert.Len(t, model.nodes, 2)
	assert.NotNil(t, model.nodes["step1"])
	assert.NotNil(t, model.nodes["step2"])
	assert.Equal(t, status.NodeNone, model.nodes["step1"].status)
	assert.Equal(t, status.NodeNone, model.nodes["step2"].status)

	// Test Init command
	cmd := model.Init()
	assert.NotNil(t, cmd)
}

func TestProgressModel_UpdateNode(t *testing.T) {
	dag := &core.DAG{
		Name: "test-dag",
		Steps: []core.Step{
			{Name: "step1"},
		},
	}

	model := NewProgressModel(dag)

	// Update node status
	node := &models.Node{
		Step:      core.Step{Name: "step1"},
		Status:    status.NodeRunning,
		StartedAt: time.Now().Format(time.RFC3339),
	}

	updatedModel, _ := model.Update(NodeUpdateMsg{Node: node})
	m := updatedModel.(ProgressModel)

	assert.Equal(t, status.NodeRunning, m.nodes["step1"].status)
	assert.False(t, m.nodes["step1"].startTime.IsZero())
}

func TestProgressModel_UpdateStatus(t *testing.T) {
	dag := &core.DAG{Name: "test-dag"}
	model := NewProgressModel(dag)

	dagRunStatus := &models.DAGRunStatus{
		DAGRunID: "run-123",
		Params:   "KEY=value",
		Status:   status.Running,
	}

	updatedModel, _ := model.Update(StatusUpdateMsg{Status: dagRunStatus})
	m := updatedModel.(ProgressModel)

	assert.Equal(t, "run-123", m.dagRunID)
	assert.Equal(t, "KEY=value", m.params)
	assert.Equal(t, status.Running, m.status.Status)
}

func TestProgressModel_WindowResize(t *testing.T) {
	dag := &core.DAG{Name: "test-dag"}
	model := NewProgressModel(dag)

	// Test window resize
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m := updatedModel.(ProgressModel)

	assert.Equal(t, 120, m.width)
	assert.Equal(t, 40, m.height)
}

func TestProgressModel_View(t *testing.T) {
	dag := &core.DAG{
		Name: "test-dag",
		Steps: []core.Step{
			{Name: "step1"},
			{Name: "step2"},
		},
	}

	model := NewProgressModel(dag)
	model.width = 80
	model.height = 24

	// Test initial view
	view := model.View()
	assert.Contains(t, view, "DAG: test-dag")
	assert.Contains(t, view, "Status:")
	assert.Contains(t, view, "Progress:")
	assert.Contains(t, view, "0% (0/2 steps)")
	assert.Contains(t, view, "Press Ctrl+C to stop")
}

func TestProgressModel_Finalize(t *testing.T) {
	dag := &core.DAG{Name: "test-dag"}
	model := NewProgressModel(dag)

	// Test finalize
	updatedModel, cmd := model.Update(FinalizeMsg{})
	m := updatedModel.(ProgressModel)

	assert.True(t, m.finalized)
	assert.NotNil(t, cmd) // Should return tea.Quit
}

func TestProgressModel_ProgressCalculation(t *testing.T) {
	dag := &core.DAG{
		Name: "test-dag",
		Steps: []core.Step{
			{Name: "step1"},
			{Name: "step2"},
			{Name: "step3"},
			{Name: "step4"},
		},
	}

	model := NewProgressModel(dag)
	model.width = 80

	// Mark some steps as completed
	model.nodes["step1"].status = status.NodeSuccess
	model.nodes["step2"].status = status.NodeError
	model.nodes["step3"].status = status.NodeRunning
	model.nodes["step4"].status = status.NodeNone

	view := model.View()

	// 2 out of 4 steps completed (50%)
	assert.Contains(t, view, "50% (2/4 steps)")
	assert.Contains(t, view, "Currently Running:")
	assert.Contains(t, view, "step3")
}

func TestProgressModel_StatusFormatting(t *testing.T) {
	dag := &core.DAG{Name: "test-dag"}
	model := NewProgressModel(dag)

	tests := []struct {
		status   status.Status
		expected string
	}{
		{status.Success, "Success ✓"},
		{status.Error, "Failed ✗"},
		{status.Running, "Running ●"},
		{status.Cancel, "Cancelled ⚠"},
		{status.Queued, "Queued ●"},
		{status.None, "Not Started ○"},
	}

	for _, tt := range tests {
		result := model.formatStatus(tt.status)
		// Strip ANSI codes for comparison
		assert.Contains(t, result, tt.expected)
	}
}

func TestProgressModel_NodeSorting(t *testing.T) {
	// Test sorting by start time
	now := time.Now()
	nodes := []*nodeProgress{
		{
			node:      &models.Node{Step: core.Step{Name: "b"}},
			startTime: now.Add(2 * time.Second),
		},
		{
			node:      &models.Node{Step: core.Step{Name: "a"}},
			startTime: now.Add(1 * time.Second),
		},
		{
			node:      &models.Node{Step: core.Step{Name: "c"}},
			startTime: now,
		},
	}

	sortNodesByStartTime(nodes)

	assert.Equal(t, "c", nodes[0].node.Step.Name)
	assert.Equal(t, "a", nodes[1].node.Step.Name)
	assert.Equal(t, "b", nodes[2].node.Step.Name)
}

func TestProgressModel_TruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello..."},
		{"test", 4, "test"},    // String equal to maxLen should not be truncated
		{"testing", 4, "t..."}, // String longer than maxLen should be truncated
		{"", 5, ""},
	}

	for _, tt := range tests {
		result := truncateString(tt.input, tt.maxLen)
		assert.Equal(t, tt.expected, result)
	}
}

func TestProgressTeaDisplay_Integration(t *testing.T) {
	dag := &core.DAG{
		Name: "test-dag",
		Steps: []core.Step{
			{Name: "step1"},
			{Name: "step2"},
		},
	}

	display := NewProgressTeaDisplay(dag)
	require.NotNil(t, display)

	// Set DAG run info
	display.SetDAGRunInfo("run-123", "KEY=value")

	// Update a node
	node := &models.Node{
		Step:      core.Step{Name: "step1"},
		Status:    status.NodeRunning,
		StartedAt: time.Now().Format(time.RFC3339),
	}
	display.UpdateNode(node)

	// Update status
	status := &models.DAGRunStatus{
		DAGRunID: "run-123",
		Status:   status.Running,
	}
	display.UpdateStatus(status)
}

func TestProgressModel_ChildDAGsRendering(t *testing.T) {
	dag := &core.DAG{
		Name: "test-dag",
		Steps: []core.Step{
			{Name: "parent-step"},
		},
	}

	model := NewProgressModel(dag)
	model.width = 80

	// Add child DAG info
	model.nodes["parent-step"].children = []models.ChildDAGRun{
		{
			DAGRunID: "child-run-1",
			Params:   "CHILD_KEY=value",
		},
	}

	view := model.View()
	assert.Contains(t, view, "Child DAGs:")
	assert.Contains(t, view, "parent-step")
	assert.Contains(t, view, "child-run-1")
}

func TestProgressModel_ErrorDisplay(t *testing.T) {
	dag := &core.DAG{
		Name: "test-dag",
		Steps: []core.Step{
			{Name: "failing-step"},
		},
	}

	model := NewProgressModel(dag)
	model.width = 100

	// Mark step as failed with error
	model.nodes["failing-step"].status = status.NodeError
	model.nodes["failing-step"].node.Error = "Connection timeout"
	model.nodes["failing-step"].endTime = time.Now()
	model.nodes["failing-step"].startTime = time.Now().Add(-5 * time.Second)

	view := model.View()
	assert.Contains(t, view, "Recently Completed:")
	assert.Contains(t, view, "failing-step")
	assert.Contains(t, view, "Error: Connection timeout")
}
