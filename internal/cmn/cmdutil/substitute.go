package cmdutil

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// buildShellCommand creates an exec.Cmd with appropriate arguments for the shell type
func buildShellCommand(shell, cmdStr string) *exec.Cmd {
	if shell == "" {
		return exec.Command("sh", "-c", cmdStr) // nolint:gosec
	}

	shellName := strings.ToLower(filepath.Base(shell))

	switch shellName {
	case "powershell.exe", "powershell", "pwsh.exe", "pwsh":
		return exec.Command(shell, "-Command", cmdStr) // nolint:gosec
	case "cmd.exe", "cmd":
		return exec.Command(shell, "/c", cmdStr) // nolint:gosec
	default:
		return exec.Command(shell, "-c", cmdStr) // nolint:gosec
	}
}

// runCommandWithContext executes cmdStr in a shell using environment from context.
// If EnvScope is present in context, uses it; otherwise falls back to os.Environ().
func runCommandWithContext(ctx context.Context, cmdStr string) (string, error) {
	sh := GetShellCommand("")
	cmd := buildShellCommand(sh, cmdStr)

	// Use context-provided env or fall back to os.Environ()
	if scope := GetEnvScope(ctx); scope != nil {
		cmd.Env = scope.ToSlice()
	} else {
		cmd.Env = os.Environ()
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf(
			"failed to execute command %q: %w\nstderr=%s",
			cmdStr, err, stderr.String(),
		)
	}
	// Trim trailing newlines/spaces for cleanliness
	return strings.TrimSpace(stdout.String()), nil
}

// substituteCommandsWithContext is like substituteCommands but uses context for environment.
// Commands are executed with the EnvScope from context if available.
func substituteCommandsWithContext(ctx context.Context, input string) (string, error) {
	var result strings.Builder     // final output
	var cmdBuilder strings.Builder // accumulates text inside a command
	inCommand := false             // whether we're currently capturing a command

	runes := []rune(input)
	i := 0
	for i < len(runes) {
		r := runes[i]

		// Check if current rune is a backslash and next rune is a backtick => treat it as a "command-delim" backtick
		if r == '\\' && i+1 < len(runes) && runes[i+1] == '`' {
			// Skip the escaped backtick
			result.WriteString("\\`")
			i += 2 // advance past the backslash
			continue
		}

		if r == '`' {
			// Toggle command mode
			if inCommand {
				if cmdBuilder.Len() == 0 {
					// If the command is empty leave the backticks as-is
					result.WriteString("``")
				} else {
					// We are closing a command - use context-aware execution
					output, err := runCommandWithContext(ctx, cmdBuilder.String())
					if err != nil {
						return "", err
					}
					result.WriteString(output)
				}
				cmdBuilder.Reset()
				inCommand = false
			} else {
				// We are opening a command
				inCommand = true
			}
		} else {
			// Just a regular character
			if inCommand {
				cmdBuilder.WriteRune(r)
			} else {
				result.WriteRune(r)
			}
		}

		i++
	}

	if cmdBuilder.Len() > 0 {
		// If inCommand is true here, we never closed the command.
		// Append the command as-is to the result.
		result.WriteRune('`')
		result.WriteString(cmdBuilder.String())
	}

	return result.String(), nil
}
