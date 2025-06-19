package fileutil

import (
	"testing"
)

func TestNormalizeDAGPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty path", "", ""},
		{"dot path", ".", ""},
		{"root slash", "/", ""},
		{"leading slash", "/workflow/task1", "workflow/task1"},
		{"trailing slash", "workflow/task1/", "workflow/task1"},
		{"multiple slashes", "workflow//task1", "workflow/task1"},
		{"normal path", "workflow/task1", "workflow/task1"},
		{"nested path", "data/extract/users", "data/extract/users"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeDAGPath(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeDAGPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSplitDAGPath(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedPrefix string
		expectedName   string
	}{
		{"empty path", "", "", ""},
		{"no prefix", "task1", "", "task1"},
		{"single level prefix", "workflow/task1", "workflow", "task1"},
		{"multi level prefix", "data/extract/users", "data/extract", "users"},
		{"trailing slash", "workflow/", "workflow", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix, name := SplitDAGPath(tt.input)
			if prefix != tt.expectedPrefix || name != tt.expectedName {
				t.Errorf("SplitDAGPath(%q) = (%q, %q), want (%q, %q)",
					tt.input, prefix, name, tt.expectedPrefix, tt.expectedName)
			}
		})
	}
}

func TestJoinDAGPath(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		dagName  string
		expected string
	}{
		{"empty both", "", "", ""},
		{"empty prefix", "", "task1", "task1"},
		{"empty name", "workflow", "", "workflow"},
		{"normal join", "workflow", "task1", "workflow/task1"},
		{"nested prefix", "data/extract", "users", "data/extract/users"},
		{"prefix with slashes", "/workflow/", "task1", "workflow/task1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JoinDAGPath(tt.prefix, tt.dagName)
			if result != tt.expected {
				t.Errorf("JoinDAGPath(%q, %q) = %q, want %q",
					tt.prefix, tt.dagName, result, tt.expected)
			}
		})
	}
}

func TestGetParentPrefix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty prefix", "", ""},
		{"root level", "workflow", ""},
		{"single nested", "workflow/extract", "workflow"},
		{"multi nested", "data/pipeline/extract", "data/pipeline"},
		{"with slashes", "/workflow/extract/", "workflow"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetParentPrefix(tt.input)
			if result != tt.expected {
				t.Errorf("GetParentPrefix(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsValidDAGPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"empty path", "", false},
		{"valid simple", "task1", true},
		{"valid with prefix", "workflow/task1", true},
		{"valid nested", "data/extract/users", true},
		{"invalid double dot", "../task1", false},
		{"invalid backslash", "workflow\\task1", false},
		{"invalid colon", "workflow:task1", false},
		{"invalid asterisk", "workflow*task1", false},
		{"invalid question", "workflow?task1", false},
		{"invalid quote", "workflow\"task1", false},
		{"invalid less than", "workflow<task1", false},
		{"invalid greater than", "workflow>task1", false},
		{"invalid pipe", "workflow|task1", false},
		{"invalid null", "workflow\x00task1", false},
		{"invalid empty component", "workflow//task1", false},
		{"invalid dot component", "workflow/./task1", false},
		{"invalid leading space", " workflow/task1", true},            // space in path is ok
		{"invalid trailing space component", "workflow/task1 ", true}, // space in path is ok
		{"invalid space in component", "workflow/ task1", false},      // leading/trailing space in component is not ok
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidDAGPath(tt.input)
			if result != tt.expected {
				t.Errorf("IsValidDAGPath(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
