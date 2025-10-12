package core

import (
	"testing"
)

func TestStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		expected int
	}{
		{"StatusNone", None, 0},
		{"StatusRunning", Running, 1},
		{"StatusError", Error, 2},
		{"StatusCancel", Cancel, 3},
		{"StatusSuccess", Success, 4},
		{"StatusQueued", Queued, 5},
		{"StatusPartialSuccess", PartialSuccess, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.status) != tt.expected {
				t.Errorf("Expected %s = %d, got %d", tt.name, tt.expected, int(tt.status))
			}
		})
	}
}

func TestNodeStatusConstants(t *testing.T) {
	tests := []struct {
		name       string
		nodeStatus NodeStatus
		expected   int
	}{
		{"NodeStatusNone", NodeNone, 0},
		{"NodeStatusRunning", NodeRunning, 1},
		{"NodeStatusError", NodeError, 2},
		{"NodeStatusCancel", NodeCancel, 3},
		{"NodeStatusSuccess", NodeSuccess, 4},
		{"NodeStatusSkipped", NodeSkipped, 5},
		{"NodeStatusPartialSuccess", NodePartialSuccess, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.nodeStatus) != tt.expected {
				t.Errorf("Expected %s = %d, got %d", tt.name, tt.expected, int(tt.nodeStatus))
			}
		})
	}
}

func TestStatusString(t *testing.T) {
	tests := []struct {
		status   Status
		expected string
	}{
		{None, "not started"},
		{Running, "running"},
		{Error, "failed"},
		{Cancel, "cancelled"},
		{Success, "finished"},
		{Queued, "queued"},
		{PartialSuccess, "partial success"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.status.String() != tt.expected {
				t.Errorf("Expected Status(%d).String() = %q, got %q", tt.status, tt.expected, tt.status.String())
			}
		})
	}
}

func TestNodeStatusString(t *testing.T) {
	tests := []struct {
		nodeStatus NodeStatus
		expected   string
	}{
		{NodeNone, "not started"},
		{NodeRunning, "running"},
		{NodeError, "failed"},
		{NodeCancel, "cancelled"},
		{NodeSuccess, "finished"},
		{NodeSkipped, "skipped"},
		{NodePartialSuccess, "partial success"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.nodeStatus.String() != tt.expected {
				t.Errorf("Expected NodeStatus(%d).String() = %q, got %q", tt.nodeStatus, tt.expected, tt.nodeStatus.String())
			}
		})
	}
}
