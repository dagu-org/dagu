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
			args:        []string{"start", th.DAG(t, "cmd/start.yaml").Location},
			expectedOut: []string{"Step execution started"},
		},
		{
			name:        "StartDAGWithDefaultParams",
			args:        []string{"start", th.DAG(t, "cmd/start_with_params.yaml").Location},
			expectedOut: []string{`params="[p1 p2]"`},
		},
		{
			name:        "StartDAGWithParams",
			args:        []string{"start", `--params="p3 p4"`, th.DAG(t, "cmd/start_with_params.yaml").Location},
			expectedOut: []string{`params="[p3 p4]"`},
		},
		{
			name:        "StartDAGWithParamsAfterDash",
			args:        []string{"start", th.DAG(t, "cmd/start_with_params.yaml").Location, "--", "p5", "p6"},
			expectedOut: []string{`params="[p5 p6]"`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			th.RunCommand(t, startCmd(), tc)
		})
	}
}
