package status

import (
	"testing"
)

func TestStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		expected int
	}{
		{"StatusNone", StatusNone, 0},
		{"StatusRunning", StatusRunning, 1},
		{"StatusError", StatusError, 2},
		{"StatusCancel", StatusCancel, 3},
		{"StatusSuccess", StatusSuccess, 4},
		{"StatusQueued", StatusQueued, 5},
		{"StatusPartialSuccess", StatusPartialSuccess, 6},
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
		{"NodeStatusNone", NodeStatusNone, 0},
		{"NodeStatusRunning", NodeStatusRunning, 1},
		{"NodeStatusError", NodeStatusError, 2},
		{"NodeStatusCancel", NodeStatusCancel, 3},
		{"NodeStatusSuccess", NodeStatusSuccess, 4},
		{"NodeStatusSkipped", NodeStatusSkipped, 5},
		{"NodeStatusPartialSuccess", NodeStatusPartialSuccess, 6},
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
		{StatusNone, "not started"},
		{StatusRunning, "running"},
		{StatusError, "failed"},
		{StatusCancel, "canceled"},
		{StatusSuccess, "finished"},
		{StatusQueued, "queued"},
		{StatusPartialSuccess, "partial success"},
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
		{NodeStatusNone, "not started"},
		{NodeStatusRunning, "running"},
		{NodeStatusError, "failed"},
		{NodeStatusCancel, "canceled"},
		{NodeStatusSuccess, "finished"},
		{NodeStatusSkipped, "skipped"},
		{NodeStatusPartialSuccess, "partial success"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.nodeStatus.String() != tt.expected {
				t.Errorf("Expected NodeStatus(%d).String() = %q, got %q", tt.nodeStatus, tt.expected, tt.nodeStatus.String())
			}
		})
	}
}