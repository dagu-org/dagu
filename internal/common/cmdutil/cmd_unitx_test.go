//go:build !windows
// +build !windows

package cmdutil

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKillProcessGroup(t *testing.T) {
	tests := []struct {
		name        string
		setupCmd    func() *exec.Cmd
		signal      os.Signal
		shouldError bool
	}{
		{
			name: "KillRunningProcess",
			setupCmd: func() *exec.Cmd {
				cmd := exec.Command("sleep", "10")
				cmd.SysProcAttr = &syscall.SysProcAttr{
					Setpgid: true,
					Pgid:    0,
				}
				err := cmd.Start()
				require.NoError(t, err)
				return cmd
			},
			signal:      syscall.SIGTERM,
			shouldError: false,
		},
		{
			name: "KillNilProcess",
			setupCmd: func() *exec.Cmd {
				return nil
			},
			signal:      syscall.SIGTERM,
			shouldError: false, // Should handle nil gracefully
		},
		{
			name: "KillProcessWithoutProcessField",
			setupCmd: func() *exec.Cmd {
				return &exec.Cmd{}
			},
			signal:      syscall.SIGTERM,
			shouldError: false, // Should handle nil Process gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.setupCmd()

			err := KillProcessGroup(cmd, tt.signal)

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Clean up if process was started
			if cmd != nil && cmd.Process != nil {
				_ = cmd.Wait()
			}
		})
	}
}

func TestKillMultipleProcessGroups(t *testing.T) {
	// Start multiple processes
	cmds := make(map[string]*exec.Cmd)

	// Start a few sleep processes
	for i := 0; i < 3; i++ {
		cmd := exec.Command("sleep", "10")
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
			Pgid:    0,
		}
		err := cmd.Start()
		require.NoError(t, err)
		cmds[string(rune('a'+i))] = cmd
	}

	// Add a nil command
	cmds["nil"] = nil

	// Kill all processes
	err := KillMultipleProcessGroups(cmds, syscall.SIGTERM)
	// Error might occur if process already exited, which is OK
	_ = err

	// Verify processes are terminated
	for name, cmd := range cmds {
		if cmd != nil && cmd.Process != nil {
			// Give process time to die
			done := make(chan error, 1)
			go func(c *exec.Cmd) {
				done <- c.Wait()
			}(cmd)

			select {
			case <-done:
				// Process terminated
			case <-time.After(1 * time.Second):
				t.Errorf("Process %s did not terminate", name)
			}
		}
	}
}

func TestSetupCommand_Unix(t *testing.T) {
	cmd := exec.Command("echo", "test")

	// Verify SysProcAttr is nil before setup
	assert.Nil(t, cmd.SysProcAttr)

	// Setup command
	setupCommand(cmd)

	// Verify SysProcAttr is set correctly
	require.NotNil(t, cmd.SysProcAttr)
	assert.True(t, cmd.SysProcAttr.Setpgid)
	assert.Equal(t, 0, cmd.SysProcAttr.Pgid)
}
