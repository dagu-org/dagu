package command

import (
	"context"
	"os/exec"
	"path/filepath"
	"slices"
)

// unixShell handles standard Unix shells (sh, bash, zsh, etc.).
// This is the default fallback for any shell not explicitly handled.
type unixShell struct{}

func (s *unixShell) Match(_ string) bool {
	// Matches everything as the default fallback
	return true
}

func (s *unixShell) Build(ctx context.Context, b *shellCommandBuilder) (*exec.Cmd, error) {
	cmd := b.Shell[0]
	args := cloneArgs(b.Shell[1:])

	// When running a command directly with a script (e.g., perl script.pl),
	// don't include shell arguments like -e
	if b.Command != "" && b.Script != "" {
		return exec.CommandContext(ctx, b.Command, append(b.Args, b.Script)...), nil // nolint: gosec
	}

	// Add errexit flag for Unix-like shells (unless user specified shell)
	if !b.UserSpecifiedShell && isUnixLikeShell(cmd) && !slices.Contains(args, "-e") {
		args = append(args, "-e")
	}

	// Add -c flag and the shell command string
	// Note: We use ShellCommandArgs (the full command string) rather than Args
	// because shell commands may contain pipes, redirections, etc. that need
	// to be interpreted by the shell
	if !slices.Contains(args, "-c") {
		args = append(args, "-c")
	}
	args = append(args, b.ShellCommandArgs)

	return exec.CommandContext(ctx, cmd, args...), nil // nolint: gosec
}

// isUnixLikeShell reports whether the named shell is known to support the -e (errexit) flag.
// It returns true for "sh", "bash", "zsh", "ksh", "ash", and "dash"; returns false for "fish", an empty string, or any unrecognized shell.
func isUnixLikeShell(shell string) bool {
	if shell == "" {
		return false
	}

	name := filepath.Base(shell)
	switch name {
	case "sh", "bash", "zsh", "ksh", "ash", "dash":
		return true
	case "fish":
		// Fish shell doesn't support -e flag
		return false
	default:
		return false
	}
}

// cloneArgs returns a shallow copy of the provided args slice so callers can modify the result without mutating the original.
func cloneArgs(args []string) []string {
	result := make([]string, len(args))
	copy(result, args)
	return result
}