package cmd_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/test"
)

func TestDryCommand(t *testing.T) {
	t.Run("DryRun", func(t *testing.T) {
		th := test.SetupCommand(t)
		tests := []test.CmdTest{
			{
				Name:        "DryRunDAG",
				Args:        []string{"dry", th.DAG(t, "cmd/dry.yaml").Location},
				ExpectedOut: []string{"Dry-run finished"},
			},
			{
				Name:        "DryRunDAGWithParams",
				Args:        []string{"dry", th.DAG(t, "cmd/dry_with_params.yaml").Location, "--params", "p3 p4"},
				ExpectedOut: []string{`[1=p3 2=p4]`},
			},
			{
				Name:        "DryRunDAGWithParamsAfterDash",
				Args:        []string{"dry", th.DAG(t, "cmd/dry_with_params.yaml").Location, "--", "p5", "p6"},
				ExpectedOut: []string{`[1=p5 2=p6]`},
			},
		}

		for _, tc := range tests {
			t.Run(tc.Name, func(t *testing.T) {
				th.RunCommand(t, cmd.CmdDry(), tc)
			})
		}
	})
}
