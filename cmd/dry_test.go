package main

import (
	"testing"
)

func TestDryCommand(t *testing.T) {
	t.Run("DryRun", func(t *testing.T) {
		th := testSetup(t)
		tests := []cmdTest{
			{
				name:        "DryRunDAG",
				args:        []string{"dry", th.DAG(t, "cmd/dry.yaml").Location},
				expectedOut: []string{"Dry-run finished"},
			},
			{
				name:        "DryRunDAGWithParams",
				args:        []string{"dry", th.DAG(t, "cmd/dry_with_params.yaml").Location, "--params", "p3 p4"},
				expectedOut: []string{`[p3 p4]`},
			},
			{
				name:        "DryRunDAGWithParamsAfterDash",
				args:        []string{"dry", th.DAG(t, "cmd/dry_with_params.yaml").Location, "--", "p5", "p6"},
				expectedOut: []string{`[p5 p6]`},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				th.RunCommand(t, dryCmd(), tc)
			})
		}
	})
}
