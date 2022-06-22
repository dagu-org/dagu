package cmd

import (
	"testing"
)

func Test_versionCommand(t *testing.T) {
	tests := []cmdTest{
		{
			errored:     false,
			exactOutput: "0.0.1\n",
		},
	}

	for _, v := range tests {
		cmd := versionCmd
		runCmdTestOutput(cmd, v, t)
	}
}
