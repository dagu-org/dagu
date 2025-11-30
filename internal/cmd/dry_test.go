package cmd_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/test"
)

func TestDryCommand(t *testing.T) {
	t.Run("DryRun", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagDry := th.DAG(t, `steps:
  - name: "1"
    command: "true"
`)

		dagDryWithParams := th.DAG(t, `params: "p1 p2"
steps:
  - name: "1"
    command: 'echo "params is $1 and $2"'
`)

		tests := []test.CmdTest{
			{
				Name:        "DryRunDAG",
				Args:        []string{"dry", dagDry.Location},
				ExpectedOut: []string{"Dry-run completed"},
			},
			{
				Name:        "DryRunDAGWithParams",
				Args:        []string{"dry", dagDryWithParams.Location, "--params", "p3 p4"},
				ExpectedOut: []string{`[1=p3 2=p4]`},
			},
			{
				Name:        "DryRunDAGWithParamsAfterDash",
				Args:        []string{"dry", dagDryWithParams.Location, "--", "p5", "p6"},
				ExpectedOut: []string{`1=p5 2=p6`},
			},
		}

		for _, tc := range tests {
			t.Run(tc.Name, func(t *testing.T) {
				th.RunCommand(t, cmd.Dry(), tc)
			})
		}
	})
}
