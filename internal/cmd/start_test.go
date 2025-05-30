package cmd_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/test"
)

func TestStartCommand(t *testing.T) {
	th := test.SetupCommand(t)

	tests := []test.CmdTest{
		{
			Name:        "StartDAG",
			Args:        []string{"start", th.DAG(t, "cmd/start.yaml").Location},
			ExpectedOut: []string{"Step started"},
		},
		{
			Name:        "StartDAGWithDefaultParams",
			Args:        []string{"start", th.DAG(t, "cmd/start_with_params.yaml").Location},
			ExpectedOut: []string{`params="[1=p1 2=p2]"`},
		},
		{
			Name:        "StartDAGWithParams",
			Args:        []string{"start", `--params="p3 p4"`, th.DAG(t, "cmd/start_with_params.yaml").Location},
			ExpectedOut: []string{`params="[1=p3 2=p4]"`},
		},
		{
			Name:        "StartDAGWithParamsAfterDash",
			Args:        []string{"start", th.DAG(t, "cmd/start_with_params.yaml").Location, "--", "p5", "p6"},
			ExpectedOut: []string{`params="[1=p5 2=p6]"`},
		},
		{
			Name:        "StartDAGWithRequestID",
			Args:        []string{"start", th.DAG(t, "cmd/start_with_dagrun_id.yaml").Location, "--run-id", "01967be2-3bb9-78ef-a54f-5e258df2d020"},
			ExpectedOut: []string{"01967be2-3bb9-78ef-a54f-5e258df2d020"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			th.RunCommand(t, cmd.CmdStart(), tc)
		})
	}
}
