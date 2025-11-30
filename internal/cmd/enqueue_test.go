package cmd_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/test"
)

func TestEnqueueCommand(t *testing.T) {
	th := test.SetupCommand(t)

	dagEnqueue := th.DAG(t, `steps:
  - name: "1"
    command: "true"
`)

	dagEnqueueWithParams := th.DAG(t, `params: "p1 p2"
steps:
  - name: "1"
    command: "echo \"params is $1 and $2\""
`)

	tests := []test.CmdTest{
		{
			Name:        "Enqueue",
			Args:        []string{"enqueue", dagEnqueue.Location},
			ExpectedOut: []string{"Enqueued"},
		},
		{
			Name:        "EnqueueWithParams",
			Args:        []string{"enqueue", `--params="p3 p4"`, dagEnqueueWithParams.Location},
			ExpectedOut: []string{`params="[1=p3 2=p4]"`},
		},
		{
			Name:        "StartDAGWithParamsAfterDash",
			Args:        []string{"enqueue", dagEnqueueWithParams.Location, "--", "p5", "p6"},
			ExpectedOut: []string{`params="[1=p5 2=p6`},
		},
		{
			Name:        "EnqueueWithDAGRunID",
			Args:        []string{"enqueue", `--run-id="test-dag-run"`, dagEnqueue.Location},
			ExpectedOut: []string{"test-dag-run"},
		},
		{
			Name:        "EnqueueWithQueueOverride",
			Args:        []string{"enqueue", `--queue="custom-queue"`, dagEnqueue.Location},
			ExpectedOut: []string{"Enqueued"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			th.RunCommand(t, cmd.Enqueue(), tc)
		})
	}
}
