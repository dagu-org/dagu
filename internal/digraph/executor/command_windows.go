//go:build windows
// +build windows

package executor

import (
	"os"
	"os/exec"
)

// SetupCommand configures Windows-specific command attributes
func SetupCommand(cmd *exec.Cmd) {
	setupCommand(cmd)
}

// setupCommand configures Windows-specific command attributes
func setupCommand(cmd *exec.Cmd) {
	// Windows doesn't support process groups in the same way as Unix
	// No special configuration needed
}

// killProcessGroup kills the process on Windows systems
func killProcessGroup(cmd *exec.Cmd, sig os.Signal) error {
	if cmd != nil && cmd.Process != nil {
		return cmd.Process.Kill()
	}
	return nil
}

// killMultipleProcessGroups kills multiple processes on Windows systems
func killMultipleProcessGroups(cmds map[string]*exec.Cmd, sig os.Signal) error {
	var lastErr error
	for _, cmd := range cmds {
		if cmd != nil && cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil {
				lastErr = err
			}
		}
	}
	return lastErr
}
