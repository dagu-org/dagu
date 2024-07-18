package cmd

import (
	"testing"

	"github.com/dagu-dev/dagu/internal/test"
)

func TestStartCommand(t *testing.T) {
	setup := test.SetupTest(t)
	defer setup.Cleanup()

	tests := []cmdTest{
		{
			args:        []string{"start", testDAGFile("success.yaml")},
			expectedOut: []string{"1 finished"},
		},
		{
			args:        []string{"start", testDAGFile("params.yaml")},
			expectedOut: []string{"params is p1 and p2"},
		},
		{
			args: []string{
				"start",
				`--params="p3 p4"`,
				testDAGFile("params.yaml"),
			},
			expectedOut: []string{"params is p3 and p4"},
		},
	}

	for _, tc := range tests {
		testRunCommand(t, startCmd(), tc)
	}
}
