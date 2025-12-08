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
			name:             "EmptyArgs",
			dir:              tmpDir,
			shellCommandArgs: "",
			expected:         "",
		},
		{
			name:             "BatScriptExists_AddsPrefix",
			dir:              tmpDir,
			shellCommandArgs: "test.bat",
			expected:         ".\\test.bat",
		},
		{
			name:             "CmdScriptExists_AddsPrefix",
			dir:              tmpDir,
			shellCommandArgs: "test.cmd",
			expected:         ".\\test.cmd",
		},
		{
			name:             "Ps1ScriptExists_AddsPrefix",
			dir:              tmpDir,
			shellCommandArgs: "test.ps1",
			expected:         ".\\test.ps1",
		},
		{
			name:             "ScriptWithArgs_AddsPrefix",
			dir:              tmpDir,
			shellCommandArgs: "test.bat arg1 arg2",
			expected:         ".\\test.bat arg1 arg2",
		},
		{
			name:             "AlreadyHasDotBackslashPrefix",
			dir:              tmpDir,
			shellCommandArgs: ".\\test.bat",
			expected:         ".\\test.bat",
		},
		{
			name:             "AlreadyHasDotSlashPrefix",
			dir:              tmpDir,
			shellCommandArgs: "./test.bat",
			expected:         "./test.bat",
		},
		{
			name:             "RelativePathWithSubdirBackslash_NoChange",
			dir:              tmpDir,
			shellCommandArgs: "subdir\\sub.bat",
			expected:         "subdir\\sub.bat",
		},
		{
			name:             "RelativePathWithForwardSlash_NoChange",
			dir:              tmpDir,
			shellCommandArgs: "subdir/sub.bat",
			expected:         "subdir/sub.bat",
		},
		{
			name:             "CommandWithoutScriptExtension_NoChange",
			dir:              tmpDir,
			shellCommandArgs: "python script.py",
			expected:         "python script.py",
		},
		{
			name:             "NonExistentScript_NoChange",
			dir:              tmpDir,
			shellCommandArgs: "nonexistent.bat",
			expected:         "nonexistent.bat",
		},
		{
			name:             "EchoCommand_NoChange",
			dir:              tmpDir,
			shellCommandArgs: "echo hello",
			expected:         "echo hello",
		},
		{
			name:             "AbsolutePathWindows_NoChange",
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
