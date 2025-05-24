package cmd_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/test"
)

func TestDequeueCommand(t *testing.T) {
	th := test.SetupCommand(t)

	dag := th.DAG(t, "cmd/dequeue.yaml")

	// Enqueue the DAG first
	th.RunCommand(t, cmd.CmdEnqueue(), test.CmdTest{
		Name: "Enqueue",
		Args: []string{"enqueue", "--workflow-id", "test-workflow", dag.Location},
	})

	// Now test the dequeue command
	th.RunCommand(t, cmd.CmdDequeue(), test.CmdTest{
		Name:        "Dequeue",
		Args:        []string{"dequeue", "--workflow-name", "dequeue", "--workflow-id=test-workflow"},
		ExpectedOut: []string{"Dequeued workflow"},
	})
}
