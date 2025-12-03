package command

import (
	"context"
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

func (s *cmdShell) Build(ctx context.Context, b *shellCommandBuilder) (*exec.Cmd, error) {
	cmd := b.Shell[0]
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
