package command

import (
	"context"
	"os"
	"os/exec"
	"slices"
)

// cmdShell handles Windows cmd.exe.
type cmdShell struct{}

func (s *cmdShell) Match(name string) bool {
	switch name {
	case "cmd.exe", "cmd":
		return true
	default:
		return false
	}
}

// defaultCmdExePath is the standard Windows cmd.exe location.
const defaultCmdExePath = `C:\Windows\System32\cmd.exe`

// resolveCmdPath returns the full path to cmd.exe if the shell is specified
// as just "cmd" or "cmd.exe". It first tries the default path, then falls back
// to using SystemRoot environment variable if Windows is installed elsewhere.
// This ensures failing command execution due to golang security restrictions is avoided.
func resolveCmdPath(shell string) string {
	if shell != "cmd" && shell != "cmd.exe" {
		return shell
	}

	// Try the default path first (most common case)
	if _, err := os.Stat(defaultCmdExePath); err == nil {
		return defaultCmdExePath
	}

	// Fallback to SystemRoot if Windows is installed on a different drive
	if systemRoot := os.Getenv("SystemRoot"); systemRoot != "" {
		systemRootCmd := systemRoot + `\System32\cmd.exe`
		if _, err := os.Stat(systemRootCmd); err == nil {
			return systemRootCmd
		}
	}

	return shell
}

func (s *cmdShell) Build(ctx context.Context, b *shellCommandBuilder) (*exec.Cmd, error) {
	cmd := resolveCmdPath(b.Shell[0])
	args := cloneArgs(b.Shell[1:])

	// When running a command directly with a script, don't include cmd.exe arguments
	if b.Command != "" && b.Script != "" {
		args := cloneArgs(b.Args)
		if b.Script != "" {
			args = append(args, b.Script)
		}
		return exec.CommandContext(ctx, b.Command, args...), nil // nolint: gosec
	}

	args = append(args, b.Args...)

	// cmd.exe uses /c instead of -c
	if !slices.Contains(args, "/c") && !slices.Contains(args, "/C") {
		args = append(args, "/c")
	}

	args = append(args, b.normalizeScriptPath())

	return exec.CommandContext(ctx, cmd, args...), nil // nolint: gosec
}
