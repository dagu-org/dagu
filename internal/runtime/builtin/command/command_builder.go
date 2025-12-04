package command

import (
	"context"
	"fmt"
	"os/exec"
)

// shellCommandBuilder holds the configuration for building shell commands.
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
// It delegates to the appropriate Shell implementation from the registry.
func (b *shellCommandBuilder) Build(ctx context.Context) (*exec.Cmd, error) {
	if len(b.Shell) == 0 {
		return nil, fmt.Errorf("shell command is required")
	}

	shell := findShell(b.Shell[0])
	return shell.Build(ctx, b)
}
