package command

import (
	"context"
	"os/exec"
	"slices"
	"strings"
)

// nixShell handles nix-shell with package management support.
type nixShell struct{}

var _ Shell = (*nixShell)(nil)

func (s *nixShell) Match(name string) bool {
	return name == "nix-shell"
}

func (s *nixShell) Build(ctx context.Context, b *shellCommandBuilder) (*exec.Cmd, error) {
	cmd := b.Shell[0]
	args := cloneArgs(b.Shell[1:])

	// Add packages
	for _, pkg := range b.ShellPackages {
		args = append(args, "-p", pkg)
	}

	// Add pure mode if not already specified
	if !slices.Contains(args, "--pure") && !slices.Contains(args, "--impure") {
		args = append(args, "--pure")
	}

	if !slices.Contains(args, "--run") {
		args = append(args, "--run")
	}

	// When using nix-shell with a direct command and script,
	// run the command inside nix-shell
	// e.g., nix-shell -p python --run "python script.py"
	if b.Command != "" && b.Script != "" {
		cmdParts := []string{b.Command}
		cmdParts = append(cmdParts, b.Args...)
		cmdParts = append(cmdParts, b.Script)
		cmdStr := strings.Join(cmdParts, " ")

		// Apply set -e for error handling
		if !b.UserSpecifiedShell && !strings.HasPrefix(cmdStr, "set -e") {
			cmdStr = "set -e; " + cmdStr
		}

		return exec.CommandContext(ctx, cmd, append(args, cmdStr)...), nil // nolint: gosec
	}

	// When running just a script file with nix-shell (no explicit command)
	// e.g., nix-shell --run "set -e; ./script.sh"
	if b.Script != "" {
		scriptCmd := b.Script
		if !b.UserSpecifiedShell && !strings.HasPrefix(scriptCmd, "set -e") {
			scriptCmd = "set -e; " + scriptCmd
		}
		return exec.CommandContext(ctx, cmd, append(args, scriptCmd)...), nil // nolint: gosec
	}

	// For shell command args, prepend "set -e;" for errexit
	shellCmdArgs := b.ShellCommandArgs
	if !b.UserSpecifiedShell && shellCmdArgs != "" && !strings.HasPrefix(shellCmdArgs, "set -e") {
		shellCmdArgs = "set -e; " + shellCmdArgs
	}

	if shellCmdArgs != "" {
		args = append(args, shellCmdArgs)
	}

	return exec.CommandContext(ctx, cmd, args...), nil // nolint: gosec
}
