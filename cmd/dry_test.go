package cmd

import (
	"os"
	"testing"
)

func TestDryCommand(t *testing.T) {
	tmpDir, _, _ := setupTest(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	tests := []cmdTest{
		{
			args:        []string{"dry", testDAGFile("dry.yaml")},
			expectedOut: []string{"Starting DRY-RUN"},
		},
	}
	for _, tc := range tests {
		testRunCommand(t, dryCmd(), tc)
	}
}
