package runtime

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
			name:      "SimpleExitStatus",
			errStr:    "exit status 1",
			wantCode:  1,
			wantFound: true,
		},
		{
			name:      "ExitStatusWithMessage",
			errStr:    "command failed: exit status 127",
			wantCode:  127,
			wantFound: true,
		},
		{
			name:      "ExitStatusInMiddleOfString",
			errStr:    "failed to run command: exit status 42: command not found",
			wantCode:  42,
			wantFound: true,
		},
		{
			name:      "MultipleExitStatusOccurrences",
			errStr:    "first: exit status 1, second: exit status 2",
			wantCode:  2,
			wantFound: true,
		},
		{
			name:      "ExitStatusWithTrailingText",
			errStr:    "exit status 255 (permission denied)",
			wantCode:  255,
			wantFound: true,
		},
		{
			name:      "ZeroExitStatus",
			errStr:    "exit status 0",
			wantCode:  0,
			wantFound: true,
		},
		{
			name:      "LargeExitCode",
			errStr:    "exit status 32768",
			wantCode:  32768,
			wantFound: true,
		},
		{
			name:      "NegativeNumbersNotParsed",
			errStr:    "exit status -1",
			wantCode:  0,
			wantFound: false,
		},
		{
			name:      "NoExitStatus",
			errStr:    "command failed",
			wantCode:  0,
			wantFound: false,
		},
		{
			name:      "ExitStatusWithoutNumber",
			errStr:    "exit status",
			wantCode:  0,
			wantFound: false,
		},
		{
			name:      "ExitStatusWithNonNumeric",
			errStr:    "exit status abc",
			wantCode:  0,
			wantFound: false,
		},
		{
			name:      "EmptyString",
			errStr:    "",
			wantCode:  0,
			wantFound: false,
		},
		{
			name:      "ExitStatusWithDecimal",
			errStr:    "exit status 1.5",
			wantCode:  1,
			wantFound: true,
		},
		{
			name:      "WrappedErrorWithExitStatus",
			errStr:    "error: failed to execute: /bin/sh: command not found: exit status 127",
			wantCode:  127,
			wantFound: true,
		},
		{
			name:      "DockerContainerExitStatus",
			errStr:    "Error response from daemon: Container abc123 exited with non-zero exit status 137",
			wantCode:  137,
			wantFound: true,
		},
		{
			name:      "MultiLineErrorWithExitStatus",
			errStr:    "Command failed:\n  Output: error\n  exit status 3",
			wantCode:  3,
			wantFound: true,
		},
		{
			name:      "ExitStatusAtBeginning",
			errStr:    "exit status 5: command failed",
			wantCode:  5,
			wantFound: true,
		},
		{
			name:      "MixedNumbersInError",
			errStr:    "step 1 failed: exit status 2 after 3 retries",
			wantCode:  2,
			wantFound: true,
		},
		{
			name:      "ExitStatusWithLeadingZeros",
			errStr:    "exit status 007",
			wantCode:  7,
			wantFound: true,
		},
		{
			name:      "SignalTerminationNotExitStatus",
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
	t.Run("VeryLongErrorString", func(t *testing.T) {
		longPrefix := strings.Repeat("error ", 1000)
		errStr := longPrefix + "exit status 99"
		code, found := parseExitCodeFromError(errStr)
		assert.True(t, found)
		assert.Equal(t, 99, code)
	})

	// Test with multiple exit status patterns
	t.Run("NestedExitStatusPatterns", func(t *testing.T) {
		errStr := "wrapper: exit status 1: inner command: exit status 255"
		code, found := parseExitCodeFromError(errStr)
		assert.True(t, found)
		assert.Equal(t, 255, code) // Should get the last one
	})

	// Test Unicode characters
	t.Run("UnicodeInErrorString", func(t *testing.T) {
		errStr := "コマンドが失敗しました: exit status 128"
		code, found := parseExitCodeFromError(errStr)
		assert.True(t, found)
		assert.Equal(t, 128, code)
	})
}
