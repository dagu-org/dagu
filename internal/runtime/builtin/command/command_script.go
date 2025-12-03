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
// setupScript creates a temporary executable script file in workDir containing
// the provided script after applying shell-specific preprocessing (for example,
// PowerShell error-handling directives). The file extension is chosen from the
// supplied shell; the function returns the created file path or an error if file
// creation, writing, syncing, or permission setting fails.
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

// preprocessScript returns the script content adjusted for the shell indicated by ext.
// For ".ps1" it prepends PowerShell directives that make cmdlet errors and non-zero exit codes stop execution; for other extensions it returns the original script.
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

// createDirectCommand returns an *exec.Cmd that invokes cmd with the provided args.
// If scriptFile is non-empty it is appended to the argument list. The returned command
// is bound to ctx.
func createDirectCommand(ctx context.Context, cmd string, args []string, scriptFile string) *exec.Cmd {
	if scriptFile != "" {
		clonedArgs := cloneArgs(args)
		clonedArgs = append(clonedArgs, scriptFile)
		return exec.CommandContext(ctx, cmd, clonedArgs...) // nolint: gosec
	}
	return exec.CommandContext(ctx, cmd, args...) // nolint: gosec
}

// validateCommandStep checks that a Step has a valid command configuration.
// It considers a step valid when it provides a Command, a Script, both, or a non-nil SubDAG.
// Returns core.ErrStepCommandIsRequired when none of those are present.
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