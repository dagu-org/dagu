package cmdutil

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runCommand executes cmdStr in a shell, capturing stdout (and ignoring stderr).
func runCommand(cmdStr string) (string, error) {
	sh := GetShellCommand("")
	cmd := buildShellCommand(sh, cmdStr)
	cmd.Env = os.Environ()

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

// buildShellCommand creates an exec.Cmd with appropriate arguments for the shell type
func buildShellCommand(shell, cmdStr string) *exec.Cmd {
	if shell == "" {
		// Fallback to basic execution
		return exec.Command("sh", "-c", cmdStr) // nolint:gosec
	}

	// Extract just the executable name for comparison
	shellName := strings.ToLower(filepath.Base(shell))

	switch shellName {
	case "powershell.exe", "powershell":
		// PowerShell (Windows PowerShell)
		return exec.Command(shell, "-Command", cmdStr) // nolint:gosec

	case "pwsh.exe", "pwsh":
		// PowerShell Core (cross-platform)
		return exec.Command(shell, "-Command", cmdStr) // nolint:gosec

	case "cmd.exe", "cmd":
		// Windows Command Prompt
		return exec.Command(shell, "/c", cmdStr) // nolint:gosec

	default:
		// Unix-like shells (sh, bash, zsh, etc.)
		return exec.Command(shell, "-c", cmdStr) // nolint:gosec
	}
}

// substituteCommands scans for backtick-delimited commands, including "escaped" backticks
// (i.e. a backslash immediately before a backtick). If we see "\`", we treat it as a real
// backtick delimiter, not a literal backslash + backtick. Commands are executed via runCommand().
func substituteCommands(input string) (string, error) {
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
					// We are closing a command
					output, err := runCommand(cmdBuilder.String())
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

	// If inCommand is true here, we never closed the command.
	// Decide how to handle unclosed commands. Here, we do nothing special.

	return result.String(), nil
}
