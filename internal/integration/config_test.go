package integration_test

import (
	"os"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestDAGExecution(t *testing.T) {
	t.Parallel()

	t.Run("Depends", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: "1"
    command: "echo 1"
  - name: "2"
    depends: "1"
    command: "echo 2"
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
	})

	t.Run("Pipe", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `params:
  - NAME: "foo"
steps:
  - name: step1
    command: echo hello $NAME | xargs echo
    output: OUT1
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": "hello foo",
		})
	})

	t.Run("DotEnv", func(t *testing.T) {
		th := test.Setup(t)

		// Create dotenv files
		dotenv1Path := test.TestdataPath(t, "integration/dotenv1")
		dotenv2Path := test.TestdataPath(t, "integration/dotenv2")

		dag := th.DAG(t, `dotenv:
  - `+dotenv1Path+`
  - `+dotenv2Path+`
steps:
  - name: step1
    command: echo "${ENV1} ${ENV2}"
    output: OUT1 #123 abc
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": "123 abc",
		})
	})

	t.Run("NamedParams", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `params:
  NAME: "Dagu"
  AGE: 30

steps:
  - name: Hello
    command: echo $NAME
    output: OUT1
  - name: Name
    command: echo Hello, $NAME
    depends: Hello
    output: OUT2
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": "Dagu",
			"OUT2": "Hello, Dagu",
		})
	})

	t.Run("NamedParamsList", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `params:
  - NAME: "Dagu"
  - AGE: 30

steps:
  - name: Hello
    command: echo $NAME
    output: OUT1
  - name: Name
    command: echo Hello, $NAME
    depends: Hello
    output: OUT2
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": "Dagu",
			"OUT2": "Hello, Dagu",
		})
	})

	t.Run("PositionalParams", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `params: "foo bar"

steps:
  - name: step1
    output: OUT1
    command: echo '$1' is $1, '$2' is $2
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": []test.Contains{
				"$1 is foo",
				"$2 is bar",
			},
		})
	})

	t.Run("PositionalParamsScript", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `params: "foo bar"

steps:
  - name: step1
    output: OUT1
    script: |
      echo '$1' is $1, '$2' is ${2}
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": []test.Contains{
				"$1 is foo",
				"$2 is bar",
			},
		})
	})

	t.Run("Script", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `params:
  - NAME: "foo"
steps:
  - name: step1
    script: |
      echo 1 2 3
    output: OUT1
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": "1 2 3",
		})
	})

	t.Run("RegexPrecondition", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: test
    command: echo abc run def
    output: OUT1
  - name: test2
    command: echo match
    depends: test
    output: OUT2
    precondition:
      - condition: "$OUT1"
        expected: "re:^abc.*def$"
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": "abc run def",
			"OUT2": "match",
		})
	})

	t.Run("JSON", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: get config
    command: |
      echo '{"port": 8080, "host": "localhost"}'
    output: CONFIG

  - name: start server
    command: echo "Starting server at ${CONFIG.host}:${CONFIG.port}"
    output: OUT1
    depends: get config
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": "Starting server at localhost:8080",
		})
	})

	t.Run("EnvVar", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `env:
  - DATA_DIR: /tmp/dagu_test_integration
  - PROCESS_DATE: "`+"`"+`date '+%Y%m%d_%H%M%S'`+"`"+`"

steps:
  - name: output_file
    command: echo foo
    stdout: "${DATA_DIR}_${PROCESS_DATE}"
  - name: make_tmp_file
    command: cat ${DATA_DIR}_${PROCESS_DATE}
    output: OUT1
    depends: output_file
    precondition:
      - condition: "${DATA_DIR}_${PROCESS_DATE}"
        expected: "re:[0-9]{8}_[0-9]{6}"
  - name: cleanup
    command: rm ${DATA_DIR}_${PROCESS_DATE}
    depends: make_tmp_file
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": "foo",
		})
	})

	t.Run("EnvScript", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `env:
  - "E1": foo
  - "E2": bar

steps:
  - name: step1
    output: OUT1
    script: |
      echo E1 is $E1, E2 is $E2
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": []test.Contains{
				"E1 is foo",
				"E2 is bar",
			},
		})
	})

	t.Run("SpecialVars", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: step1
    command: echo $DAG_RUN_LOG_FILE
    output: OUT1
  - name: step2
    command: echo $DAG_RUN_STEP_STDOUT_FILE
    output: OUT2
  - name: step3
    command: echo $DAG_RUN_STEP_NAME
    output: OUT3
  - name: step4
    command: sh
    output: OUT4
    script: |
      echo $DAG_NAME
  - name: step5
    command: bash
    output: OUT5
    script: |
      echo $DAG_RUN_ID
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": test.NotEmpty{},
			"OUT2": test.NotEmpty{},
			"OUT3": test.NotEmpty{},
			"OUT4": test.NotEmpty{},
			"OUT5": test.NotEmpty{},
		})
	})

	t.Run("JQ", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: extract_value
    executor: jq
    command: .user.name # Get user name from JSON
    output: NAME
    script: |
      {
        "user": {
          "name": "John",
          "age": 30
        }
      }
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
		dag.AssertOutputs(t, map[string]any{
			"NAME": `"John"`,
		})
	})

	t.Run("JSONVar", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: get_config
    command: |
      echo '{"port": 8080, "host": "localhost"}'
    output: CONFIG

  - name: start_server
    command: echo "Starting server at ${CONFIG.host}:${CONFIG.port}"
    output: OUT1
    depends: get_config
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": "Starting server at localhost:8080",
		})
	})

	t.Run("PerlScript", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: step1
    command: perl
    script: |
      use strict;
      use warnings;
      print("Hello World\n");
    output: OUT1
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": "Hello World",
		})
	})

	t.Run("Workdir", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `env:
  - WORKDIR: $HOME
  - TILDE: ~/
steps:
  - name: step1
    dir: $TILDE
    command: echo $PWD
    output: OUT1

  - name: step2
    dir: $WORKDIR
    command: echo $PWD
    output: OUT2
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": os.ExpandEnv("$HOME"),
			"OUT2": os.ExpandEnv("$HOME"),
		})
	})

	t.Run("Issue-810", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `params: bar
steps:
  - name: step1
    command: echo start
    output: OUT1 # "start"
  - name: foo
    command: echo foo
    depends:
      - step1
    output: OUT2 # "foo"
    preconditions:
      - condition: $OUT1 # should be "start"
        expected: start
  - name: bar
    command: echo bar
    depends:
      - step1
    output: OUT3 # "bar"
    preconditions:
      - condition: "$1" # should be "bar"
        expected: bar
  - name: baz
    command: echo baz
    depends:
      - foo
      - bar
    output: OUT4 # "baz"
    preconditions:
      - condition: "$1" # should be "bar"
        expected: bar
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": "start",
			"OUT2": "foo",
			"OUT3": "bar",
			"OUT4": "baz",
		})
	})

	t.Run("ShellOptions", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: step1
    description: test step
    command: |
      echo 'hello world' && ls -al /
    shell: bash -o errexit -o xtrace -o pipefail -c
    output: OUT1
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, status.Success)
		dag.AssertOutputs(t, map[string]any{
			"OUT1": []test.Contains{
				"hello world",
			},
		})
	})
}

func TestCallSubDAG(t *testing.T) {
	th := test.Setup(t)

	// Use multi-document YAML to include both parent and sub DAG
	dagContent := `steps:
  - name: step1
    run: sub
    params: "SUB_P1=foo"
    output: OUT1
  - name: step2
    command: echo "${OUT1.outputs.OUT}"
    output: OUT2
    depends: [step1]
---
name: sub
params:
  SUB_P1: xyz
steps:
  - name: step1
    command: echo $SUB_P1
    output: OUT
`
	dag := th.DAG(t, dagContent)
	agent := dag.Agent()

	agent.RunSuccess(t)

	dag.AssertLatestStatus(t, status.Success)
	dag.AssertOutputs(t, map[string]any{
		"OUT2": "foo",
	})
}

func TestNestedThreeLevelDAG(t *testing.T) {
	th := test.Setup(t)

	// Create the grandchild DAG as a separate file
	th.CreateDAGFile(t, th.Config.Paths.DAGsDir, "nested_grand_child", []byte(`
params:
  PARAM: VALUE
steps:
  - name: grand_child
    command: "echo value is ${PARAM}"
    output: OUTPUT
`))

	// Create parent and child DAGs using multi-document YAML
	dagContent := `steps:
  - name: child
    run: nested_child
    params: "PARAM=123"
    output: CHILD_OUTPUT
  - name: output
    command: "echo ${CHILD_OUTPUT.outputs.OUTPUT}"
    output: OUT1
    depends:
      - child
---
name: nested_child
params:
  PARAM: VALUE
steps:
  - name: child
    run: nested_grand_child
    params: "PARAM=${PARAM}"
    output: GRAND_CHILD_OUTPUT
  - name: output
    command: "echo ${GRAND_CHILD_OUTPUT.outputs.OUTPUT}"
    depends:
      - child
    output: OUTPUT
`
	dag := th.DAG(t, dagContent)
	agent := dag.Agent()

	agent.RunSuccess(t)

	dag.AssertLatestStatus(t, status.Success)
	dag.AssertOutputs(t, map[string]any{
		"OUT1": "value is 123",
	})
}

// TestSkippedPreconditions verifies that steps with unmet preconditions are skipped.
func TestSkippedPreconditions(t *testing.T) {
	t.Parallel()

	// Setup the test helper with the integration DAGs directory.
	th := test.Setup(t)
	// Load the DAG from inline YAML.
	dag := th.DAG(t, `type: graph  # Use graph mode to avoid implicit dependencies
steps:
  - name: run-step
    command: echo "executed"
    output: OUT_RUN
  - name: skip-step
    command: echo "should not execute"
    preconditions:
      - condition: "`+"`"+`echo no`+"`"+`"
        expected: "yes"
    output: OUT_SKIP1
  - name: skip-step2
    command: echo "should execute"
    preconditions:
      - condition: "`+"`"+`echo yes`+"`"+`"
        expected: "yes"
    output: OUT_SKIP2
`)
	agent := dag.Agent()

	// Run the DAG and expect it to complete successfully.
	agent.RunSuccess(t)

	// Assert that the final status is successful.
	dag.AssertLatestStatus(t, status.Success)

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
	th := test.Setup(t)
	// Load the DAG from inline YAML.
	dag := th.DAG(t, `steps:
  - name: start
    command: echo "start"
    output: START
  - name: branch1
    command: echo "branch1"
    depends: start
    output: BRANCH1
  - name: branch2
    command: echo "branch2"
    depends: start
    output: BRANCH2
  - name: merge
    command: echo "merge"
    depends:
      - branch1
      - branch2
    output: MERGE
  - name: final
    command: echo "final"
    depends: merge
    output: FINAL
`)
	agent := dag.Agent()

	// Run the DAG and expect it to complete successfully.
	agent.RunSuccess(t)

	// Assert that the final status is successful.
	dag.AssertLatestStatus(t, status.Success)

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

	th := test.Setup(t)

	dag := th.DAG(t, `steps:
  - name: "1"
    command: "sleep 3"
  - name: "2"
    command: "sleep 3"
    depends: "1"
`)
	agent := dag.Agent()

	go func() {
		err := agent.Run(agent.Context)
		require.NoError(t, err, "failed to run agent")
	}()

	dag.AssertCurrentStatus(t, status.Running)

	dagRunStatus, err := dag.DAGRunMgr.GetLatestStatus(dag.Context, dag.DAG)
	require.NoError(t, err, "failed to get latest status")

	// Check the first node is in progress
	require.Equal(t, status.NodeRunning.String(), dagRunStatus.Nodes[0].Status.String(), "first node should be in progress")
	// Check the second node is not started
	require.Equal(t, status.NodeNone.String(), dagRunStatus.Nodes[1].Status.String(), "second node should not be started")

	// Wait for the first node to finish
	time.Sleep(time.Second * 2)

	dag.AssertCurrentStatus(t, status.Running)

	// Check the progress of the nodes
	dagRunStatus, err = dag.DAGRunMgr.GetLatestStatus(dag.Context, dag.DAG)
	require.NoError(t, err, "failed to get latest status")

	// Assert that the dag-run is still running
	require.Equal(t, status.Running.String(), dagRunStatus.Status.String(), "dag-run should be running")

	// Check the first node is finished
	require.Equal(t, status.NodeSuccess.String(), dagRunStatus.Nodes[0].Status.String(), "first node should be finished")
	// Check the second node is in progress
	require.Equal(t, status.NodeRunning.String(), dagRunStatus.Nodes[1].Status.String(), "second node should be in progress")

	// Wait for all nodes to finish
	dag.AssertLatestStatus(t, status.Success)

	// Check the second node is finished
	dagRunStatus, err = dag.DAGRunMgr.GetLatestStatus(dag.Context, dag.DAG)
	require.NoError(t, err, "failed to get latest status")

	require.Equal(t, status.NodeSuccess.String(), dagRunStatus.Nodes[1].Status.String(), "second node should be finished")
}
