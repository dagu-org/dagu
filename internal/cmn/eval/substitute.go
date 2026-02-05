package eval

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/cmn/cmdutil"
)

// buildShellCommand creates an exec.Cmd with appropriate arguments for the shell type.
func buildShellCommand(shell, cmdStr string) *exec.Cmd {
	if shell == "" {
		return exec.Command("sh", "-c", cmdStr) //nolint:gosec
	}

	switch strings.ToLower(filepath.Base(shell)) {
	case "powershell.exe", "powershell", "pwsh.exe", "pwsh":
		return exec.Command(shell, "-Command", cmdStr) //nolint:gosec
	case "cmd.exe", "cmd":
		return exec.Command(shell, "/c", cmdStr) //nolint:gosec
	default:
		return exec.Command(shell, "-c", cmdStr) //nolint:gosec
	}
}

// runCommandWithContext executes cmdStr in a shell using the EnvScope from context,
// falling back to os.Environ() when no scope is present.
func runCommandWithContext(ctx context.Context, cmdStr string) (string, error) {
	cmd := buildShellCommand(cmdutil.GetShellCommand(""), cmdStr)

	if scope := GetEnvScope(ctx); scope != nil {
		cmd.Env = scope.ToSlice()
	} else {
		cmd.Env = os.Environ()
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf(
			"failed to execute command %q: %w\nstderr=%s",
			cmdStr, err, stderr.String(),
		)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// substituteCommandsWithContext replaces backtick-delimited commands in input
// with their execution output, using the EnvScope from context if available.
func substituteCommandsWithContext(ctx context.Context, input string) (string, error) {
	var result strings.Builder
	var cmdBuilder strings.Builder
	inCommand := false

	runes := []rune(input)
	i := 0
	for i < len(runes) {
		r := runes[i]

		// Escaped backtick: preserve literally and skip ahead.
		if r == '\\' && i+1 < len(runes) && runes[i+1] == '`' {
			result.WriteString("\\`")
			i += 2
			continue
		}

		if r == '`' {
			if inCommand {
				if cmdBuilder.Len() == 0 {
					// Empty backticks: preserve as-is.
					result.WriteString("``")
				} else {
					output, err := runCommandWithContext(ctx, cmdBuilder.String())
					if err != nil {
						return "", err
					}
					result.WriteString(output)
				}
				cmdBuilder.Reset()
				inCommand = false
			} else {
				inCommand = true
			}
		} else if inCommand {
			cmdBuilder.WriteRune(r)
		} else {
			result.WriteRune(r)
		}

		i++
	}

	// Unclosed backtick: append the partial command as-is.
	if cmdBuilder.Len() > 0 {
		result.WriteRune('`')
		result.WriteString(cmdBuilder.String())
	}

	return result.String(), nil
}
