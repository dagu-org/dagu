package scheduler

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseExitCodeFromError(t *testing.T) {
	tests := []struct {
		name      string
		errStr    string
		wantCode  int
		wantFound bool
	}{
		{
			name:      "simple exit status",
			errStr:    "exit status 1",
			wantCode:  1,
			wantFound: true,
		},
		{
			name:      "exit status with message",
			errStr:    "command failed: exit status 127",
			wantCode:  127,
			wantFound: true,
		},
		{
			name:      "exit status in middle of string",
			errStr:    "failed to run command: exit status 42: command not found",
			wantCode:  42,
			wantFound: true,
		},
		{
			name:      "multiple exit status occurrences",
			errStr:    "first: exit status 1, second: exit status 2",
			wantCode:  2,
			wantFound: true,
		},
		{
			name:      "exit status with trailing text",
			errStr:    "exit status 255 (permission denied)",
			wantCode:  255,
			wantFound: true,
		},
		{
			name:      "zero exit status",
			errStr:    "exit status 0",
			wantCode:  0,
			wantFound: true,
		},
		{
			name:      "large exit code",
			errStr:    "exit status 32768",
			wantCode:  32768,
			wantFound: true,
		},
		{
			name:      "negative numbers not parsed",
			errStr:    "exit status -1",
			wantCode:  0,
			wantFound: false,
		},
		{
			name:      "no exit status",
			errStr:    "command failed",
			wantCode:  0,
			wantFound: false,
		},
		{
			name:      "exit status without number",
			errStr:    "exit status",
			wantCode:  0,
			wantFound: false,
		},
		{
			name:      "exit status with non-numeric",
			errStr:    "exit status abc",
			wantCode:  0,
			wantFound: false,
		},
		{
			name:      "empty string",
			errStr:    "",
			wantCode:  0,
			wantFound: false,
		},
		{
			name:      "exit status with decimal",
			errStr:    "exit status 1.5",
			wantCode:  1,
			wantFound: true,
		},
		{
			name:      "wrapped error with exit status",
			errStr:    "error: failed to execute: /bin/sh: command not found: exit status 127",
			wantCode:  127,
			wantFound: true,
		},
		{
			name:      "docker container exit status",
			errStr:    "Error response from daemon: Container abc123 exited with non-zero exit status 137",
			wantCode:  137,
			wantFound: true,
		},
		{
			name:      "multi-line error with exit status",
			errStr:    "Command failed:\n  Output: error\n  exit status 3",
			wantCode:  3,
			wantFound: true,
		},
		{
			name:      "exit status at beginning",
			errStr:    "exit status 5: command failed",
			wantCode:  5,
			wantFound: true,
		},
		{
			name:      "mixed numbers in error",
			errStr:    "step 1 failed: exit status 2 after 3 retries",
			wantCode:  2,
			wantFound: true,
		},
		{
			name:      "exit status with leading zeros",
			errStr:    "exit status 007",
			wantCode:  7,
			wantFound: true,
		},
		{
			name:      "signal termination (not exit status)",
			errStr:    "signal: killed",
			wantCode:  0,
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCode, gotFound := parseExitCodeFromError(tt.errStr)
			assert.Equal(t, tt.wantFound, gotFound, "found mismatch")
			if gotFound {
				assert.Equal(t, tt.wantCode, gotCode, "exit code mismatch")
			}
		})
	}
}

func TestParseExitCodeFromError_EdgeCases(t *testing.T) {
	// Test with very long strings
	t.Run("very long error string", func(t *testing.T) {
		longPrefix := strings.Repeat("error ", 1000)
		errStr := longPrefix + "exit status 99"
		code, found := parseExitCodeFromError(errStr)
		assert.True(t, found)
		assert.Equal(t, 99, code)
	})

	// Test with multiple exit status patterns
	t.Run("nested exit status patterns", func(t *testing.T) {
		errStr := "wrapper: exit status 1: inner command: exit status 255"
		code, found := parseExitCodeFromError(errStr)
		assert.True(t, found)
		assert.Equal(t, 255, code) // Should get the last one
	})

	// Test Unicode characters
	t.Run("unicode in error string", func(t *testing.T) {
		errStr := "コマンドが失敗しました: exit status 128"
		code, found := parseExitCodeFromError(errStr)
		assert.True(t, found)
		assert.Equal(t, 128, code)
	})
}
