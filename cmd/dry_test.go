package cmd

import (
	"testing"
)

func TestDryCommand(t *testing.T) {
	t.Run("DryRun", func(t *testing.T) {
		setup := setupTest(t)
		defer setup.cleanup()

		tests := []cmdTest{
			{
				args:        []string{"dry", testDAGFile("dry.yaml")},
				expectedOut: []string{"Starting DRY-RUN"},
			},
		}

		for _, tc := range tests {
			testRunCommand(t, dryCmd(), tc)
		}
	})
}
