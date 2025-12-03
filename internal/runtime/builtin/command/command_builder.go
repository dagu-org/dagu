package command

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// shellCommandBuilder builds shell commands for different shell types.
type shellCommandBuilder struct {
	Dir                string
	Command            string
	Args               []string
	Shell              []string // Shell command, e.g., ["/bin/sh"]
	ShellCommandArgs   string
	ShellPackages      []string
	Script             string
	UserSpecifiedShell bool // If true, don't auto-add -e flag
}

// Build constructs an exec.Cmd based on the shell type.
func (b *shellCommandBuilder) Build(ctx context.Context) (*exec.Cmd, error) {
	if len(b.Shell) == 0 {
		return nil, fmt.Errorf("shell command is required")
	}

	cmd := b.Shell[0]
	args := make([]string, len(b.Shell)-1)
	copy(args, b.Shell[1:])

	// Extract just the executable name for comparison
	cmdName := strings.ToLower(filepath.Base(cmd))

	switch cmdName {
	case "nix-shell":
		return b.buildNixShellCommand(ctx, cmd, args)

	case "powershell.exe", "powershell", "pwsh.exe", "pwsh":
		return b.buildPowerShellCommand(ctx, cmd, args)

	case "cmd.exe", "cmd":
		return b.buildCmdCommand(ctx, cmd, args)

	default:
		return b.buildDefaultShellCommand(ctx, cmd, args)
	}
}

// buildNixShellCommand builds a command for nix-shell.
func (b *shellCommandBuilder) buildNixShellCommand(ctx context.Context, cmd string, args []string) (*exec.Cmd, error) {
	for _, pkg := range b.ShellPackages {
		args = append(args, "-p", pkg)
	}
	args = append(args, "--pure")
	if !slices.Contains(args, "--run") {
		args = append(args, "--run")
	}

	if b.Command != "" && b.Script != "" {
		// When using nix-shell with a direct command and script,
		// we need to run the command inside nix-shell, not pass nix-shell args to the command
		cmdParts := []string{b.Command}
		cmdParts = append(cmdParts, b.Args...)
		cmdParts = append(cmdParts, b.Script)
		cmdStr := strings.Join(cmdParts, " ")

		// Apply set -e if needed
		if !b.UserSpecifiedShell && !strings.HasPrefix(cmdStr, "set -e") {
			cmdStr = "set -e; " + cmdStr
		}

		return exec.CommandContext(ctx, cmd, append(args, cmdStr)...), nil // nolint: gosec
	}

	// For nix-shell, prepend "set -e;" to enable errexit (unless user specified shell)
	shellCmdArgs := b.ShellCommandArgs
	if !b.UserSpecifiedShell && shellCmdArgs != "" && !strings.HasPrefix(shellCmdArgs, "set -e") {
		shellCmdArgs = "set -e; " + shellCmdArgs
	}

	// Construct the command with the shell command and the packages
	return exec.CommandContext(ctx, cmd, append(args, shellCmdArgs)...), nil // nolint: gosec
}

// buildPowerShellCommand builds a command for PowerShell (both Windows PowerShell and PowerShell Core).
func (b *shellCommandBuilder) buildPowerShellCommand(ctx context.Context, cmd string, args []string) (*exec.Cmd, error) {
	if b.Command != "" && b.Script != "" {
		// When running a command directly with a script, don't include PowerShell arguments
		return exec.CommandContext(ctx, b.Command, append(b.Args, b.Script)...), nil // nolint: gosec
	}

	args = append(args, b.Args...)
	// PowerShell uses -Command instead of -c
	if !slices.Contains(args, "-Command") && !slices.Contains(args, "-C") {
		args = append(args, "-Command")
	}

	args = append(args, b.normalizeScriptPath())

	// nolint: gosec
	return exec.CommandContext(ctx, cmd, args...), nil
}

// buildCmdCommand builds a command for Windows cmd.exe.
func (b *shellCommandBuilder) buildCmdCommand(ctx context.Context, cmd string, args []string) (*exec.Cmd, error) {
	if b.Command != "" && b.Script != "" {
		// When running a command directly with a script, don't include cmd.exe arguments
		return exec.CommandContext(ctx, b.Command, append(b.Args, b.Script)...), nil // nolint: gosec
	}

	args = append(args, b.Args...)

	// cmd.exe uses /c instead of -c
	if !slices.Contains(args, "/c") && !slices.Contains(args, "/C") {
		args = append(args, "/c")
	}
	args = append(args, b.normalizeScriptPath())

	// nolint: gosec
	return exec.CommandContext(ctx, cmd, args...), nil
}

// buildDefaultShellCommand builds a command for Unix-like shells (sh, bash, zsh, etc.).
func (b *shellCommandBuilder) buildDefaultShellCommand(ctx context.Context, cmd string, args []string) (*exec.Cmd, error) {
	if b.Command != "" && b.Script != "" {
		// When running a command directly with a script (e.g., perl script.pl),
		// don't include shell arguments like -e
		return exec.CommandContext(ctx, b.Command, append(b.Args, b.Script)...), nil // nolint: gosec
	}

	// Add errexit flag for Unix-like shells (unless user specified shell)
	if !b.UserSpecifiedShell && isUnixLikeShell(cmd) && !slices.Contains(args, "-e") {
		args = append(args, "-e")
	}

	args = append(args, b.Args...)
	if !slices.Contains(args, "-c") {
		args = append(args, "-c")
	}
	args = append(args, b.ShellCommandArgs)

	// nolint: gosec
	return exec.CommandContext(ctx, cmd, args...), nil
}

// isUnixLikeShell returns true if the shell supports -e flag.
func isUnixLikeShell(shell string) bool {
	if shell == "" {
		return false
	}

	// Extract just the executable name (handle full paths)
	shellName := filepath.Base(shell)

	switch shellName {
	case "sh", "bash", "zsh", "ksh", "ash", "dash":
		return true
	case "fish":
		// Fish shell doesn't support -e flag
		return false
	default:
		return false
	}
}
