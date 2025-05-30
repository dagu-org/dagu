package cmd_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/test"
)

func TestEnqueueCommand(t *testing.T) {
	th := test.SetupCommand(t)

	tests := []test.CmdTest{
		{
			Name:        "Enqueue",
			Args:        []string{"enqueue", th.DAG(t, "cmd/enqueue.yaml").Location},
			ExpectedOut: []string{"Enqueued"},
		},
		{
			Name:        "EnqueueWithParams",
			Args:        []string{"enqueue", `--params="p3 p4"`, th.DAG(t, "cmd/enqueue_with_params.yaml").Location},
			ExpectedOut: []string{`params="[1=p3 2=p4]"`},
		},
		{
			Name:        "StartDAGWithParamsAfterDash",
			Args:        []string{"enqueue", th.DAG(t, "cmd/enqueue_with_params.yaml").Location, "--", "p5", "p6"},
			ExpectedOut: []string{`params="[1=p5 2=p6]"`},
		},
		{
			Name:        "EnqueueWithDAGRunID",
			Args:        []string{"enqueue", `--run-id="test-dag-run"`, th.DAG(t, "cmd/enqueue.yaml").Location},
			ExpectedOut: []string{"test-dag-run"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			th.RunCommand(t, cmd.CmdEnqueue(), tc)
		})
	}
}
