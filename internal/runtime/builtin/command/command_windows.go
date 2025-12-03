//go:build windows

package command

import (
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/common/fileutil"
)

// normalizeScriptPath adds ".\" prefix to scripts in the working directory
// that lack a path prefix.
//
// This is Windows-specific: PowerShell and cmd.exe (when invoked programmatically)
// don't search the current directory by default, unlike interactive cmd prompts.
func (b *shellCommandBuilder) normalizeScriptPath() string {
	if b.ShellCommandArgs == "" {
		return ""
	}
	parts := strings.SplitN(b.ShellCommandArgs, " ", 2)
	first := parts[0]

	// Skip if already has a path component (absolute or relative)
	if filepath.IsAbs(first) || strings.ContainsAny(first, `/\`) {
		return b.ShellCommandArgs
	}

	// Only add prefix for Windows script extensions to avoid incorrectly
	// prefixing commands that happen to have a same-named file in the directory.
	// For example, if user runs "python script.py" and there's a file called
	// "python" in the directory, we shouldn't prefix it.
	if !isWindowsScriptExtension(first) {
		return b.ShellCommandArgs
	}

	// Check if the file exists in the working directory
	file := filepath.Join(b.Dir, first)
	if fileutil.IsFile(file) {
		return ".\\" + b.ShellCommandArgs
	}
	return b.ShellCommandArgs
}

// isWindowsScriptExtension reports whether filename has a Windows script extension (".bat", ".cmd", or ".ps1").
func isWindowsScriptExtension(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".bat", ".cmd", ".ps1":
		return true
	default:
		return false
	}
}