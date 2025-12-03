package command

import (
	"context"
	"os/exec"
	"slices"
	"strings"
)

// nixShell handles nix-shell with package management support.
type nixShell struct{}

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

	// nix-shell runs in pure mode by default
	args = append(args, "--pure")

	if !slices.Contains(args, "--run") {
		args = append(args, "--run")
	}

	// When using nix-shell with a direct command and script,
	// run the command inside nix-shell
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

	// For shell command args, prepend "set -e;" for errexit
	shellCmdArgs := b.ShellCommandArgs
	if !b.UserSpecifiedShell && shellCmdArgs != "" && !strings.HasPrefix(shellCmdArgs, "set -e") {
		shellCmdArgs = "set -e; " + shellCmdArgs
	}

	return exec.CommandContext(ctx, cmd, append(args, shellCmdArgs)...), nil // nolint: gosec
}
