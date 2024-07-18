package cmd

import (
	"testing"

	"github.com/dagu-dev/dagu/internal/test"
)

func TestDryCommand(t *testing.T) {
	t.Run("DryRun", func(t *testing.T) {
		setup := test.Setup(t)
		defer setup.Cleanup()

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
