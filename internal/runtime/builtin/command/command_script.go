package command

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/core"
)

// setupScript creates a temporary script file and returns its path.
// It handles shell-specific preprocessing (e.g., PowerShell error handling).
func setupScript(workDir, script string, shell []string) (string, error) {
	// Determine file extension based on shell
	shellCmd := ""
	if len(shell) > 0 {
		shellCmd = shell[0]
	}
	ext := cmdutil.GetScriptExtension(shellCmd)
	pattern := "dagu_script-*" + ext

	file, err := os.CreateTemp(workDir, pattern)
	if err != nil {
		return "", fmt.Errorf("failed to create script file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	// Apply shell-specific preprocessing
	script = preprocessScript(script, ext)

	if _, err = file.WriteString(script); err != nil {
		return "", fmt.Errorf("failed to write script to file: %w", err)
	}

	if err = file.Sync(); err != nil {
		return "", fmt.Errorf("failed to sync script file: %w", err)
	}

	// Add execute permissions to the script file
	if err = os.Chmod(file.Name(), 0750); err != nil { // nolint: gosec
		return "", fmt.Errorf("failed to set execute permissions on script file: %w", err)
	}

	return file.Name(), nil
}

// preprocessScript applies shell-specific preprocessing to the script content.
func preprocessScript(script, ext string) string {
	switch ext {
	case ".ps1":
		// For PowerShell scripts, prepend error handling settings:
		// $ErrorActionPreference = 'Stop' - stops on cmdlet errors
		// $PSNativeCommandUseErrorActionPreference = $true - stops on non-zero exit codes (PowerShell 7.4+)
		return "$ErrorActionPreference = 'Stop'\n$PSNativeCommandUseErrorActionPreference = $true\n" + script
	default:
		return script
	}
}

// createDirectCommand creates a command that runs directly without a shell.
func createDirectCommand(ctx context.Context, cmd string, args []string, scriptFile string) *exec.Cmd {
	arguments := make([]string, len(args))
	copy(arguments, args)

	if scriptFile != "" {
		arguments = append(arguments, scriptFile)
	}

	// nolint: gosec
	return exec.CommandContext(ctx, cmd, arguments...)
}

// validateCommandStep validates that a step has the required command configuration.
func validateCommandStep(step core.Step) error {
	switch {
	case step.Command != "" && step.Script != "":
		// Both command and script provided - valid
	case step.Command != "" && step.Script == "":
		// Command only - valid
	case step.Command == "" && step.Script != "":
		// Script only - valid
	case step.SubDAG != nil:
		// Sub DAG - valid
	default:
		return core.ErrStepCommandIsRequired
	}

	return nil
}
