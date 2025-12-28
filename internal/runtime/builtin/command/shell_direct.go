package command

import (
	"context"
	"fmt"
	"os/exec"
)

// directShell executes commands directly without a shell wrapper.
// Use this when you don't need shell features (variable expansion, piping, globbing)
// and want to avoid shell overhead or quoting complexities.
//
// Example usage in YAML:
//
//	shell: direct
//	steps:
//	  - name: run-python
//	    command: [/usr/bin/python, -u, script.py]
//
// Note: The command must be specified as an array [cmd, arg1, arg2].
// String commands like "echo hello" are not supported with direct shell
// because they require shell parsing.
type directShell struct{}

var _ Shell = (*directShell)(nil)

func (s *directShell) Match(name string) bool {
	return name == "direct"
}

func (s *directShell) Build(ctx context.Context, b *shellCommandBuilder) (*exec.Cmd, error) {
	// For direct shell, we need Command to be set (from array syntax)
	// If only ShellCommandArgs is set (from string syntax), reject it
	// because direct execution can't parse shell command strings
	if b.Command == "" {
		if b.ShellCommandArgs != "" {
			return nil, fmt.Errorf("direct shell requires command array syntax [cmd, arg1, arg2]; string commands are not supported")
		}
		return nil, fmt.Errorf("direct shell requires 'command' field with array syntax")
	}

	// Build args: start with explicit args, append script if present
	args := cloneArgs(b.Args)
	if b.Script != "" {
		args = append(args, b.Script)
	}

	return exec.CommandContext(ctx, b.Command, args...), nil // nolint: gosec
}
