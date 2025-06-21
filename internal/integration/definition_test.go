package integration_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestDAGExecution(t *testing.T) {
	t.Parallel()

	t.Run("Depends", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "depends.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
	})

	t.Run("Pipe", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "pipe.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": "hello foo",
		})
	})

	t.Run("DotEnv", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "dotenv.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": "123 abc",
		})
	})

	t.Run("NamedParams", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "named-params.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": "Dagu",
			"OUT2": "Hello, Dagu",
		})
	})

	t.Run("NamedParamsList", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "named-params-list.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": "Dagu",
			"OUT2": "Hello, Dagu",
		})
	})

	t.Run("PositionalParams", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "positional-params.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": []test.Contains{
				"$1 is foo",
				"$2 is bar",
			},
		})
	})

	t.Run("PositionalParamsScript", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "positional-params-script.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": []test.Contains{
				"$1 is foo",
				"$2 is bar",
			},
		})
	})

	t.Run("Script", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "script.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": "1 2 3",
		})
	})

	t.Run("RegexPrecondition", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "precondition-regex.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": "abc run def",
			"OUT2": "match",
		})
	})

	t.Run("JSON", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "json.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": "Starting server at localhost:8080",
		})
	})

	t.Run("EnvVar", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "environment-var.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": "foo",
		})
	})

	t.Run("EnvScript", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "env-script.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": []test.Contains{
				"E1 is foo",
				"E2 is bar",
			},
		})
	})

	t.Run("SpecialVars", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "special-vars.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": test.NotEmpty{},
			"OUT2": test.NotEmpty{},
			"OUT3": test.NotEmpty{},
			"OUT4": test.NotEmpty{},
			"OUT5": test.NotEmpty{},
		})
	})

	t.Run("JQ", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "jq.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
		dag.AssertOutputs(t, map[string]any{
			"NAME": `"John"`,
		})
	})

	t.Run("JSONVar", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "json_var.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": "Starting server at localhost:8080",
		})
	})

	t.Run("PerlScript", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "perl-script.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": "Hello World",
		})
	})

	t.Run("Workdir", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "workdir.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": os.ExpandEnv("$HOME"),
			"OUT2": os.ExpandEnv("$HOME"),
		})
	})

	t.Run("Issue-810", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "issue-810.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": "start",
			"OUT2": "foo",
			"OUT3": "bar",
			"OUT4": "baz",
		})
	})

	t.Run("ShellOptions", func(t *testing.T) {
		th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
		dag := th.DAG(t, filepath.Join("integration", "shellopts.yaml"))
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": []test.Contains{
				"hello world",
			},
		})
	})
}

func TestNestedDAG(t *testing.T) {
	type testCase struct {
		name            string
		dag             string
		expectedOutputs map[string]any
	}

	testCases := []testCase{
		{
			name: "CallSub",
			dag:  "call-sub.yaml",
			expectedOutputs: map[string]any{
				"OUT2": "foo",
			},
		},
		{
			name: "NestedGraph",
			dag:  "nested_parent.yaml",
			expectedOutputs: map[string]any{
				"OUT1": "value is 123",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

			dag := th.DAG(t, filepath.Join("integration", tc.dag))
			agent := dag.Agent()

			agent.RunSuccess(t)

			dag.AssertLatestStatus(t, scheduler.StatusSuccess)
			dag.AssertOutputs(t, tc.expectedOutputs)
		})
	}
}

// TestSkippedPreconditions verifies that steps with unmet preconditions are skipped.
func TestSkippedPreconditions(t *testing.T) {
	t.Parallel()

	// Setup the test helper with the integration DAGs directory.
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
	// Load the DAG from testdata/integration/skipped-preconditions.yaml.
	dag := th.DAG(t, filepath.Join("integration", "skipped-preconditions.yaml"))
	agent := dag.Agent()

	// Run the DAG and expect it to complete successfully.
	agent.RunSuccess(t)

	// Assert that the final status is successful.
	dag.AssertLatestStatus(t, scheduler.StatusSuccess)

	// Verify outputs:
	// OUT_RUN should be "executed" and OUT_SKIP should be empty (indicating the step was skipped).
	dag.AssertOutputs(t, map[string]any{
		"OUT_RUN":   "executed",
		"OUT_SKIP":  "",
		"OUT_SKIP2": "should execute",
	})
}

// TestComplexDependencies verifies that a DAG with complex dependencies executes steps in the correct order.
func TestComplexDependencies(t *testing.T) {
	t.Parallel()

	// Setup the test helper with the integration DAGs directory.
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
	// Load the DAG from testdata/integration/complex-dependencies.yaml.
	dag := th.DAG(t, filepath.Join("integration", "complex-dependencies.yaml"))
	agent := dag.Agent()

	// Run the DAG and expect it to complete successfully.
	agent.RunSuccess(t)

	// Assert that the final status is successful.
	dag.AssertLatestStatus(t, scheduler.StatusSuccess)

	// Verify the outputs from each step.
	dag.AssertOutputs(t, map[string]any{
		"START":   "start",
		"BRANCH1": "branch1",
		"BRANCH2": "branch2",
		"MERGE":   "merge",
		"FINAL":   "final",
	})
}

func TestProgressingNode(t *testing.T) {
	t.Parallel()

	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	dag := th.DAG(t, filepath.Join("integration", "progress.yaml"))
	agent := dag.Agent()

	go func() {
		err := agent.Run(agent.Context)
		require.NoError(t, err, "failed to run agent")
	}()

	dag.AssertCurrentStatus(t, scheduler.StatusRunning)

	status, err := dag.DAGRunMgr.GetLatestStatus(dag.Context, dag.DAG)
	require.NoError(t, err, "failed to get latest status")

	// Check the first node is in progress
	require.Equal(t, scheduler.NodeStatusRunning.String(), status.Nodes[0].Status.String(), "first node should be in progress")
	// Check the second node is not started
	require.Equal(t, scheduler.NodeStatusNone.String(), status.Nodes[1].Status.String(), "second node should not be started")

	// Wait for the first node to finish
	time.Sleep(time.Second * 2)

	dag.AssertCurrentStatus(t, scheduler.StatusRunning)

	// Check the progress of the nodes
	status, err = dag.DAGRunMgr.GetLatestStatus(dag.Context, dag.DAG)
	require.NoError(t, err, "failed to get latest status")

	// Assert that the dag-run is still running
	require.Equal(t, scheduler.StatusRunning.String(), status.Status.String(), "dag-run should be running")

	// Check the first node is finished
	require.Equal(t, scheduler.NodeStatusSuccess.String(), status.Nodes[0].Status.String(), "first node should be finished")
	// Check the second node is in progress
	require.Equal(t, scheduler.NodeStatusRunning.String(), status.Nodes[1].Status.String(), "second node should be in progress")

	// Wait for all nodes to finish
	dag.AssertLatestStatus(t, scheduler.StatusSuccess)

	// Check the second node is finished
	status, err = dag.DAGRunMgr.GetLatestStatus(dag.Context, dag.DAG)
	require.NoError(t, err, "failed to get latest status")

	require.Equal(t, scheduler.NodeStatusSuccess.String(), status.Nodes[1].Status.String(), "second node should be finished")
}
