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
		{"StatusNone", NotStarted, 0},
		{"StatusRunning", Running, 1},
		{"StatusError", Failed, 2},
		{"StatusCancel", Canceled, 3},
		{"StatusSuccess", Succeeded, 4},
		{"StatusQueued", Queued, 5},
		{"StatusPartialSuccess", PartiallySucceeded, 6},
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
		{"NodeStatusNone", NodeNotStarted, 0},
		{"NodeStatusRunning", NodeRunning, 1},
		{"NodeStatusError", NodeFailed, 2},
		{"NodeStatusCancel", NodeCanceled, 3},
		{"NodeStatusSuccess", NodeSucceeded, 4},
		{"NodeStatusSkipped", NodeSkipped, 5},
		{"NodeStatusPartialSuccess", NodePartiallySucceeded, 6},
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
		{NotStarted, "not_started"},
		{Running, "running"},
		{Failed, "failed"},
		{Canceled, "aborted"},
		{Succeeded, "succeeded"},
		{Queued, "queued"},
		{PartiallySucceeded, "partially_succeeded"},
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
		{NodeNotStarted, "not_started"},
		{NodeRunning, "running"},
		{NodeFailed, "failed"},
		{NodeCanceled, "aborted"},
		{NodeSucceeded, "succeeded"},
		{NodeSkipped, "skipped"},
		{NodePartiallySucceeded, "partially_succeeded"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.nodeStatus.String() != tt.expected {
				t.Errorf("Expected NodeStatus(%d).String() = %q, got %q", tt.nodeStatus, tt.expected, tt.nodeStatus.String())
			}
		})
	}
}
