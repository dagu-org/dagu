package agent

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgressDisplay_SetDAGRunInfo(t *testing.T) {
	var buf bytes.Buffer
	dag := &digraph.DAG{Name: "test-dag"}
	pd := NewProgressDisplay(&buf, dag)

	dagRunID := "test-run-123"
	params := "ENV=production DEBUG=true"

	pd.SetDAGRunInfo(dagRunID, params)

	// Verify the values were set
	pd.mu.Lock()
	assert.Equal(t, dagRunID, pd.dagRunID)
	assert.Equal(t, params, pd.params)
	pd.mu.Unlock()
}

func TestProgressDisplay_UpdateStatus_WithDAGRunInfo(t *testing.T) {
	var buf bytes.Buffer
	dag := &digraph.DAG{Name: "test-dag"}
	pd := NewProgressDisplay(&buf, dag)
	pd.isEnabled = true // Enable the progress display for testing

	status := &models.DAGRunStatus{
		DAGRunID: "status-run-456",
		Params:   "BATCH_SIZE=100 MODE=async",
		Status:   scheduler.StatusRunning,
	}

	pd.UpdateStatus(status)

	// Verify the DAG run info was updated from status
	pd.mu.Lock()
	assert.Equal(t, "status-run-456", pd.dagRunID)
	assert.Equal(t, "BATCH_SIZE=100 MODE=async", pd.params)
	pd.mu.Unlock()
}

func TestProgressDisplay_RenderHeader_WithDAGRunInfo(t *testing.T) {
	tests := []struct {
		name       string
		dagName    string
		dagRunID   string
		params     string
		termWidth  int
		wantRunID  bool
		wantParams bool
	}{
		{
			name:       "Display both run ID and params",
			dagName:    "test-dag",
			dagRunID:   "abc123-def456",
			params:     "ENV=prod DEBUG=false",
			termWidth:  80,
			wantRunID:  true,
			wantParams: true,
		},
		{
			name:       "Display only run ID when no params",
			dagName:    "test-dag",
			dagRunID:   "xyz789",
			params:     "",
			termWidth:  80,
			wantRunID:  true,
			wantParams: false,
		},
		{
			name:       "No run ID or params",
			dagName:    "test-dag",
			dagRunID:   "",
			params:     "",
			termWidth:  80,
			wantRunID:  false,
			wantParams: false,
		},
		{
			name:       "Long params are truncated",
			dagName:    "test-dag",
			dagRunID:   "run-123",
			params:     "VERY_LONG_PARAMETER_NAME=value1 ANOTHER_LONG_PARAMETER=value2 YET_ANOTHER_PARAM=value3",
			termWidth:  60,
			wantRunID:  true,
			wantParams: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			dag := &digraph.DAG{Name: tt.dagName}
			pd := NewProgressDisplay(&buf, dag)
			pd.termWidth = tt.termWidth
			pd.termHeight = 24
			pd.startTime = time.Now()
			pd.dagRunID = tt.dagRunID
			pd.params = tt.params

			// Set a default status
			pd.status = &models.DAGRunStatus{
				Status: scheduler.StatusRunning,
			}

			// Render the header
			pd.renderHeader()

			output := buf.String()
			lines := strings.Split(strings.TrimSpace(output), "\n")

			// Check for Run ID line
			hasRunID := false
			hasParams := false
			for _, line := range lines {
				if strings.Contains(line, "Run ID:") {
					hasRunID = true
					if tt.wantRunID {
						assert.Contains(t, line, tt.dagRunID)
					}
				}
				if strings.Contains(line, "Params:") {
					hasParams = true
					if tt.wantParams && len(tt.params) < tt.termWidth-12 {
						// If params fit, they should be shown in full
						assert.Contains(t, line, tt.params)
					} else if tt.wantParams {
						// If params are long, they should be truncated
						assert.Contains(t, line, "...")
					}
				}
			}

			assert.Equal(t, tt.wantRunID, hasRunID, "Run ID presence mismatch")
			assert.Equal(t, tt.wantParams, hasParams, "Params presence mismatch")

			// Verify box structure is maintained
			assert.True(t, strings.HasPrefix(lines[0], "┌"), "First line should start with top-left corner")
			assert.True(t, strings.HasSuffix(lines[0], "┐"), "First line should end with top-right corner")
			assert.True(t, strings.HasPrefix(lines[len(lines)-1], "└"), "Last line should start with bottom-left corner")
			assert.True(t, strings.HasSuffix(lines[len(lines)-1], "┘"), "Last line should end with bottom-right corner")

			// Check all middle lines have proper borders
			for i := 1; i < len(lines)-1; i++ {
				assert.True(t, strings.HasPrefix(lines[i], "│"), "Line %d should start with vertical bar", i+1)
				assert.True(t, strings.HasSuffix(lines[i], "│"), "Line %d should end with vertical bar", i+1)
			}
		})
	}
}

func TestProgressDisplay_HeaderFormatting(t *testing.T) {
	var buf bytes.Buffer
	dag := &digraph.DAG{
		Name: "data-pipeline",
		Params: []string{"ENV=staging", "VERSION=2.0"},
	}
	pd := NewProgressDisplay(&buf, dag)
	pd.termWidth = 100
	pd.termHeight = 30
	pd.startTime = time.Now()
	pd.SetDAGRunInfo("pipeline-run-12345", "ENV=staging VERSION=2.0")

	// Set status
	pd.status = &models.DAGRunStatus{
		Status:   scheduler.StatusSuccess,
		DAGRunID: "pipeline-run-12345",
		Params:   "ENV=staging VERSION=2.0",
	}

	// Render header
	pd.renderHeader()

	output := buf.String()
	
	// Verify the output contains expected information
	assert.Contains(t, output, "DAG: data-pipeline")
	assert.Contains(t, output, "Status:")
	assert.Contains(t, output, "Success ✓")
	assert.Contains(t, output, "Run ID: pipeline-run-12345")
	assert.Contains(t, output, "Params: ENV=staging VERSION=2.0")
	assert.Contains(t, output, "Started:")
	assert.Contains(t, output, "Elapsed:")
}

func TestProgressDisplay_Integration(t *testing.T) {
	var buf bytes.Buffer
	dag := &digraph.DAG{
		Name: "integration-test",
		Steps: []digraph.Step{
			{Name: "setup"},
			{Name: "process"},
			{Name: "cleanup"},
		},
		Params: []string{"MODE=test"},
	}

	pd := NewProgressDisplay(&buf, dag)
	pd.termWidth = 80
	pd.termHeight = 24
	
	// Simulate agent setting DAG run info
	pd.SetDAGRunInfo("test-run-789", "MODE=test")

	// Simulate status update
	status := &models.DAGRunStatus{
		Name:     "integration-test",
		DAGRunID: "test-run-789",
		Params:   "MODE=test",
		Status:   scheduler.StatusRunning,
		Nodes: []*models.Node{
			{
				Step:   digraph.Step{Name: "setup"},
				Status: scheduler.NodeStatusSuccess,
			},
			{
				Step:   digraph.Step{Name: "process"},
				Status: scheduler.NodeStatusRunning,
			},
			{
				Step:   digraph.Step{Name: "cleanup"},
				Status: scheduler.NodeStatusNone,
			},
		},
	}

	pd.UpdateStatus(status)

	// Update nodes - make sure to update the internal node status
	pd.mu.Lock()
	for _, node := range status.Nodes {
		if np, exists := pd.nodes[node.Step.Name]; exists {
			np.status = node.Status
			np.node = node
		}
	}
	pd.mu.Unlock()

	// Render the display
	pd.mu.Lock()
	pd.renderHeader()
	pd.renderProgressBar()
	pd.mu.Unlock()

	output := buf.String()
	
	// Verify output contains all expected elements
	require.Contains(t, output, "integration-test")
	require.Contains(t, output, "test-run-789")
	require.Contains(t, output, "MODE=test")
	require.Contains(t, output, "Progress:")
	require.Contains(t, output, "(1/3 steps)") // Shows completed count
}

// Test helper function
func TestStripANSI(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "plain text",
			expected: "plain text",
		},
		{
			input:    "\033[31mred text\033[0m",
			expected: "red text",
		},
		{
			input:    "\033[1;32mgreen bold\033[0m and \033[33myellow\033[0m",
			expected: "green bold and yellow",
		},
		{
			input:    "Status: \033[32mSuccess ✓\033[0m",
			expected: "Status: Success ✓",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := stripANSI(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}