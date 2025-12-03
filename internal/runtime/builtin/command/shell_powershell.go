package command

import (
	"context"
	"os/exec"
	"slices"
)

// powerShell handles both Windows PowerShell and PowerShell Core (pwsh).
type powerShell struct{}

func (s *powerShell) Match(name string) bool {
	switch name {
	case "powershell.exe", "powershell", "pwsh.exe", "pwsh":
		return true
	default:
		return false
	}
}

func (s *powerShell) Build(ctx context.Context, b *shellCommandBuilder) (*exec.Cmd, error) {
	cmd := b.Shell[0]
	args := cloneArgs(b.Shell[1:])

	// When running a command directly with a script, don't include PowerShell arguments
	if b.Command != "" && b.Script != "" {
		return exec.CommandContext(ctx, b.Command, append(b.Args, b.Script)...), nil // nolint: gosec
	}

	args = append(args, b.Args...)

	// PowerShell uses -Command instead of -c
	if !slices.Contains(args, "-Command") && !slices.Contains(args, "-C") {
		args = append(args, "-Command")
	}

	args = append(args, b.normalizeScriptPath())

	return exec.CommandContext(ctx, cmd, args...), nil // nolint: gosec
}
