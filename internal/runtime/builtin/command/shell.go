package command

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
)

// Shell defines the interface for shell-specific command building.
// Each shell type (bash, PowerShell, cmd, nix-shell) implements this interface
// to handle its unique argument syntax and behaviors.
type Shell interface {
	// Match returns true if this shell handles the given executable name.
	// The name is lowercase and without path (e.g., "bash", "powershell.exe").
	Match(name string) bool

	// Build constructs an exec.Cmd for executing the command in this shell.
	Build(ctx context.Context, b *shellCommandBuilder) (*exec.Cmd, error)
}

// shellRegistry holds all registered shell implementations.
// Order matters: first match wins. The default Unix shell should be last.
var shellRegistry = []Shell{
	&directShell{}, // explicit no-shell execution
	&nixShell{},
	&powerShell{},
	&cmdShell{},
	&unixShell{}, // default fallback - must be last
}

// findShell returns the Shell implementation that matches the given command.
func findShell(cmd string) Shell {
	name := strings.ToLower(filepath.Base(cmd))
	for _, shell := range shellRegistry {
		if shell.Match(name) {
			return shell
		}
	}
	// Should never reach here since unixShell matches everything
	return &unixShell{}
}
