package main

import (
	"testing"
)

func TestStartCommand(t *testing.T) {
	t.Parallel()

	th := testSetup(t)

	tests := []cmdTest{
		{
			name:        "StartDAG",
			args:        []string{"start", th.DAGFile("success.yaml").Path},
			expectedOut: []string{"Step execution started"},
		},
		{
			name:        "StartDAGWithDefaultParams",
			args:        []string{"start", th.DAGFile("params.yaml").Path},
			expectedOut: []string{`params="[p1 p2]"`},
		},
		{
			name:        "StartDAGWithParams",
			args:        []string{"start", `--params="p3 p4"`, th.DAGFile("params.yaml").Path},
			expectedOut: []string{`params="[p3 p4]"`},
		},
		{
			name:        "StartDAGWithParamsAfterDash",
			args:        []string{"start", th.DAGFile("params.yaml").Path, "--", "p5", "p6"},
			expectedOut: []string{`params="[p5 p6]"`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			th.RunCommand(t, startCmd(), tc)
		})
	}
}
