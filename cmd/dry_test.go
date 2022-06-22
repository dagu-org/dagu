package cmd

import (
	"testing"
)

func Test_dryCommand(t *testing.T) {
	tests := []cmdTest{
		{
			args: []string{testConfig("cmd_dry.yaml")}, errored: false,
			output: []string{"Starting DRY-RUN"},
		},
	}

	for _, v := range tests {
		cmd := dryCmd
		runCmdTestOutput(cmd, v, t)
	}
}
