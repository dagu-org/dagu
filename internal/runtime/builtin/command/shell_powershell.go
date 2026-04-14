// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package command

import (
	"context"
	"os/exec"
	"slices"
)

// powerShell handles both Windows PowerShell and PowerShell Core (pwsh).
type powerShell struct{}

var _ Shell = (*powerShell)(nil)

func appendPowerShellStartupArgs(args []string) []string {
	if !slices.Contains(args, "-NoProfile") {
		args = append(args, "-NoProfile")
	}
	if !slices.Contains(args, "-NonInteractive") {
		args = append(args, "-NonInteractive")
	}
	return args
}

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

	// When running a command directly with a script, don't include PowerShell arguments
	// e.g., python script.py
	if b.Command != "" && b.Script != "" {
		args := cloneArgs(b.Args)
		args = append(args, b.Script)
		return exec.CommandContext(ctx, b.Command, args...), nil // nolint: gosec
	}

	// When running just a script file with PowerShell (no explicit command)
	// e.g., powershell -ExecutionPolicy Bypass -File script.ps1
	if b.Script != "" {
		args := appendPowerShellStartupArgs([]string{"-ExecutionPolicy", "Bypass", "-File", b.Script})
		return exec.CommandContext(ctx, cmd, args...), nil // nolint: gosec
	}

	// Running a command string via PowerShell
	args := appendPowerShellStartupArgs(cloneArgs(b.Shell[1:]))

	// PowerShell uses -Command instead of -c
	if !slices.Contains(args, "-Command") && !slices.Contains(args, "-C") {
		args = append(args, "-Command")
	}

	if scriptPath := b.normalizeScriptPath(); scriptPath != "" {
		args = append(args, powerShellInlineCommand(scriptPath))
	} else {
		args = append(args, powerShellInlineCommand(""))
	}

	return exec.CommandContext(ctx, cmd, args...), nil // nolint: gosec
}
