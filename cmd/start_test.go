package cmd

import "testing"

func TestStartCommand(t *testing.T) {
	tests := []cmdTest{
		{
			args:        []string{"start", testDAGFile("start.yaml")},
			expectedOut: []string{"1 finished"},
		},
		{
			args:        []string{"start", testDAGFile("start_with_params.yaml")},
			expectedOut: []string{"params is p1 and p2"},
		},
		{
			args:        []string{"start", `--params="p3 p4"`, testDAGFile("start_with_params.yaml")},
			expectedOut: []string{"params is p3 and p4"},
		},
	}

	for _, tc := range tests {
		testRunCommand(t, startCmd(), tc)
	}
}
