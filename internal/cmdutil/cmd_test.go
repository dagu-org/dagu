package cmdutil

import (
	"os/exec"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetupCommand(t *testing.T) {
	// This test verifies that setupCommand is called and sets up the command correctly
	cmd := exec.Command("echo", "test")

	// Call setupCommand (this is platform-specific)
	setupCommand(cmd)

	// On Unix, it should set process group attributes
	if runtime.GOOS != "windows" {
		require.NotNil(t, cmd.SysProcAttr)
	}
}
