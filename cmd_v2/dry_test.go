package cmd_v2

import "testing"

func TestDryCommand(t *testing.T) {
	tests := []cmdTest{
		{
			args:        []string{"dry", testDAGFile("dry.yaml")},
			expectedOut: []string{"Starting DRY-RUN"},
		},
	}

	for _, tc := range tests {
		testRunCommand(t, dryCommand(), tc)
	}
}
