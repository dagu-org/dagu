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

	type testCase struct {
		name            string
		dag             string
		expectedOutputs map[string]any
	}

	testCases := []testCase{
		{
			name: "Depends",
			dag:  "depends.yaml",
		},
		{
			name: "Pipe",
			dag:  "pipe.yaml",
			expectedOutputs: map[string]any{
				"OUT1": "hello foo",
			},
		},
		{
			name: "DotEnv",
			dag:  "dotenv.yaml",
			expectedOutputs: map[string]any{
				"OUT1": "123 abc",
			},
		},
		{
			name: "NamedParams",
			dag:  "named-params.yaml",
			expectedOutputs: map[string]any{
				"OUT1": "Dagu",
				"OUT2": "Hello, Dagu",
			},
		},
		{
			name: "NamedParamsList",
			dag:  "named-params-list.yaml",
			expectedOutputs: map[string]any{
				"OUT1": "Dagu",
				"OUT2": "Hello, Dagu",
			},
		},
		{
			name: "PositionalParams",
			dag:  "positional-params.yaml",
			expectedOutputs: map[string]any{
				"OUT1": []test.Contains{
					"$1 is foo",
					"$2 is bar",
				},
			},
		},
		{
			name: "PositionalParams",
			dag:  "positional-params-script.yaml",
			expectedOutputs: map[string]any{
				"OUT1": []test.Contains{
					"$1 is foo",
					"$2 is bar",
				},
			},
		},
		{
			name: "Script",
			dag:  "script.yaml",
			expectedOutputs: map[string]any{
				"OUT1": "1 2 3",
			},
		},
		{
			name: "RegexPrecondition",
			dag:  "precondition-regex.yaml",
			expectedOutputs: map[string]any{
				"OUT1": "abc run def",
				"OUT2": "match",
			},
		},
		{
			name: "JSON",
			dag:  "json.yaml",
			expectedOutputs: map[string]any{
				"OUT1": "Starting server at localhost:8080",
			},
		},
		{
			name: "EnvVar",
			dag:  "environment-var.yaml",
			expectedOutputs: map[string]any{
				"OUT1": "foo",
			},
		},
		{
			name: "EnvScript",
			dag:  "env-script.yaml",
			expectedOutputs: map[string]any{
				"OUT1": []test.Contains{
					"E1 is foo",
					"E2 is bar",
				},
			},
		},
		{
			name: "SpecialVars",
			dag:  "special-vars.yaml",
			expectedOutputs: map[string]any{
				"OUT1": test.NotEmpty{},
				"OUT2": test.NotEmpty{},
				"OUT3": test.NotEmpty{},
				"OUT4": test.NotEmpty{},
				"OUT5": test.NotEmpty{},
			},
		},
		{
			name: "JQ",
			dag:  "jq.yaml",
			expectedOutputs: map[string]any{
				"NAME": `"John"`,
			},
		},
		{
			name: "JSONVar",
			dag:  "json_var.yaml",
			expectedOutputs: map[string]any{
				"OUT1": "Starting server at localhost:8080",
			},
		},
		{
			name: "Script",
			dag:  "perl-script.yaml",
			expectedOutputs: map[string]any{
				"OUT1": "Hello World",
			},
		},
		{
			name: "Workdir",
			dag:  "workdir.yaml",
			expectedOutputs: map[string]any{
				"OUT1": os.ExpandEnv("$HOME"),
				"OUT2": os.ExpandEnv("$HOME"),
			},
		},
		{
			name: "Issue-810",
			dag:  "issue-810.yaml",
			expectedOutputs: map[string]any{
				"OUT1": "start",
				"OUT2": "foo",
				"OUT3": "bar",
				"OUT4": "baz",
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

	// Assert that the DAG-run is still running
	require.Equal(t, scheduler.StatusRunning.String(), status.Status.String(), "DAG-run should be running")

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
