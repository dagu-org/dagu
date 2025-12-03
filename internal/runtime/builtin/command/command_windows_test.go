//go:build windows

package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNormalizeScriptPath tests the Windows-specific path normalization.
func TestNormalizeScriptPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test script files with Windows script extensions
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.bat"), []byte("echo hello"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.cmd"), []byte("echo hello"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.ps1"), []byte("Write-Host hello"), 0o755))

	// Create a file without script extension (simulating a command collision)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "python"), []byte("fake python"), 0o755))

	// Create a script in subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "sub.bat"), []byte("echo sub"), 0o755))

	tests := []struct {
		name             string
		dir              string
		shellCommandArgs string
		expected         string
	}{
		{
			name:             "empty args",
			dir:              tmpDir,
			shellCommandArgs: "",
			expected:         "",
		},
		{
			name:             "bat script exists - adds prefix",
			dir:              tmpDir,
			shellCommandArgs: "test.bat",
			expected:         ".\\test.bat",
		},
		{
			name:             "cmd script exists - adds prefix",
			dir:              tmpDir,
			shellCommandArgs: "test.cmd",
			expected:         ".\\test.cmd",
		},
		{
			name:             "ps1 script exists - adds prefix",
			dir:              tmpDir,
			shellCommandArgs: "test.ps1",
			expected:         ".\\test.ps1",
		},
		{
			name:             "script with args - adds prefix",
			dir:              tmpDir,
			shellCommandArgs: "test.bat arg1 arg2",
			expected:         ".\\test.bat arg1 arg2",
		},
		{
			name:             "already has dot-backslash prefix",
			dir:              tmpDir,
			shellCommandArgs: ".\\test.bat",
			expected:         ".\\test.bat",
		},
		{
			name:             "already has dot-slash prefix",
			dir:              tmpDir,
			shellCommandArgs: "./test.bat",
			expected:         "./test.bat",
		},
		{
			name:             "relative path with subdir backslash - no change",
			dir:              tmpDir,
			shellCommandArgs: "subdir\\sub.bat",
			expected:         "subdir\\sub.bat",
		},
		{
			name:             "relative path with forward slash - no change",
			dir:              tmpDir,
			shellCommandArgs: "subdir/sub.bat",
			expected:         "subdir/sub.bat",
		},
		{
			name:             "command without script extension - no change even if file exists",
			dir:              tmpDir,
			shellCommandArgs: "python script.py",
			expected:         "python script.py",
		},
		{
			name:             "non-existent script - no change",
			dir:              tmpDir,
			shellCommandArgs: "nonexistent.bat",
			expected:         "nonexistent.bat",
		},
		{
			name:             "echo command - no change",
			dir:              tmpDir,
			shellCommandArgs: "echo hello",
			expected:         "echo hello",
		},
		{
			name:             "absolute path Windows - no change",
			dir:              tmpDir,
			shellCommandArgs: "C:\\Windows\\System32\\cmd.exe",
			expected:         "C:\\Windows\\System32\\cmd.exe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := &shellCommandBuilder{
				Dir:              tt.dir,
				ShellCommandArgs: tt.shellCommandArgs,
			}
			result := builder.normalizeScriptPath()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsWindowsScriptExtension tests the Windows script extension detection.
func TestIsWindowsScriptExtension(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"test.bat", true},
		{"test.BAT", true},
		{"test.cmd", true},
		{"test.CMD", true},
		{"test.ps1", true},
		{"test.PS1", true},
		{"test.exe", false},
		{"test.sh", false},
		{"test.py", false},
		{"test", false},
		{"echo", false},
		{"python", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := isWindowsScriptExtension(tt.filename)
			assert.Equal(t, tt.expected, result)
		})
	}
}
