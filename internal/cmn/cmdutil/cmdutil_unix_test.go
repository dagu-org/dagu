//go:build !windows
// +build !windows

package cmdutil

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetShellCommand_WithDAGUDefaultShell(t *testing.T) {
	// Save original env var
	originalShell := os.Getenv("DAGU_DEFAULT_SHELL")
	defer func() {
		if originalShell != "" {
			_ = os.Setenv("DAGU_DEFAULT_SHELL", originalShell)
		} else {
			_ = os.Unsetenv("DAGU_DEFAULT_SHELL")
		}
	}()

	// Test with DAGU_DEFAULT_SHELL set
	testShell := "/usr/local/bin/fish"
	_ = os.Setenv("DAGU_DEFAULT_SHELL", testShell)

	result := GetShellCommand("")
	assert.Equal(t, testShell, result)
}

func TestGetShellCommand_UnixDefaults(t *testing.T) {
	// Save original env var
	originalShell := os.Getenv("SHELL")
	originalDAGUShell := os.Getenv("DAGU_DEFAULT_SHELL")
	defer func() {
		if originalShell != "" {
			_ = os.Setenv("SHELL", originalShell)
		} else {
			_ = os.Unsetenv("SHELL")
		}
		if originalDAGUShell != "" {
			_ = os.Setenv("DAGU_DEFAULT_SHELL", originalDAGUShell)
		} else {
			_ = os.Unsetenv("DAGU_DEFAULT_SHELL")
		}
	}()

	// Clear env vars to test fallback
	_ = os.Unsetenv("SHELL")
	_ = os.Unsetenv("DAGU_DEFAULT_SHELL")

	result := GetShellCommand("")
	// Should find sh on Unix systems
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "sh")
}
