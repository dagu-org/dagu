package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestParallelExecution_ItemSources(t *testing.T) {
	const childEcho = `---
name: child-echo
params:
  - ITEM: "default"
steps:
  - command: echo "Processing $1"
    output: PROCESSED_ITEM
`

	const childProcess = `---
name: child-process
params:
  - REGION: "us-east-1"
  - VERSION: "1.0.0"
steps:
  - command: echo "Deploying version $VERSION to region $REGION"
    output: DEPLOYMENT_RESULT
`

	cases := []struct {
		name              string
		dag               string
		expectedNodes     int
		parallelNodeIndex int
		expectedChildren  int
		verify            func(*testing.T, *models.DAGRunStatus, *models.Node)
	}{
		{
			name: "simple items",
			dag: `steps:
  - run: child-echo
    parallel:
      items:
        - "item1"
        - "item2"
        - "item3"
      maxConcurrent: 3
` + childEcho,
			expectedNodes:     1,
			parallelNodeIndex: 0,
			expectedChildren:  3,
		},
		{
			name: "object items",
			dag: `steps:
  - run: child-process
    parallel:
      items:
        - REGION: us-east-1
          VERSION: "1.0.0"
        - REGION: us-west-2
          VERSION: "1.0.1"
        - REGION: eu-west-1
          VERSION: "1.0.2"
      maxConcurrent: 2
` + childProcess,
			expectedNodes:     1,
			parallelNodeIndex: 0,
			expectedChildren:  3,
			verify: func(t *testing.T, _ *models.DAGRunStatus, node *models.Node) {
				for _, child := range node.Children {
					require.Contains(t, child.Params, `"REGION"`)
					require.Contains(t, child.Params, `"VERSION"`)
				}
			},
		},
		{
			name: "variable reference",
			dag: `params:
  - ITEMS: '["alpha", "beta", "gamma", "delta"]'
steps:
  - run: child-echo
    parallel: ${ITEMS}
` + childEcho,
			expectedNodes:     1,
			parallelNodeIndex: 0,
			expectedChildren:  4,
		},
		{
			name: "space separated",
			dag: `env:
  - SERVERS: "server1 server2 server3"
steps:
  - run: child-echo
    parallel: ${SERVERS}
` + childEcho,
			expectedNodes:     1,
			parallelNodeIndex: 0,
			expectedChildren:  3,
		},
		{
			name: "direct variable",
			dag: `env:
  - ITEMS: '["task1", "task2", "task3"]'
steps:
  - run: child-with-output
    parallel: $ITEMS
  - name: aggregate-results
    command: echo "Completed parallel tasks"
    output: FINAL_RESULT
` + `---
name: child-with-output
params:
  - TASK: "default"
steps:
  - command: |
      echo "Processing task: $1"
      echo "TASK_RESULT_$1"
    output: TASK_OUTPUT
  - echo "Task $1 completed with output ${TASK_OUTPUT}"
`,
			expectedNodes:     2,
			parallelNodeIndex: 0,
			expectedChildren:  3,
			verify: func(t *testing.T, dagStatus *models.DAGRunStatus, _ *models.Node) {
				require.Greater(t, len(dagStatus.Nodes), 1, "node index out of range")
				aggregate := dagStatus.Nodes[1]
				require.Equal(t, status.NodeSuccess, aggregate.Status)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			th := test.Setup(t)
			dag := th.DAG(t, tc.dag)
			agent := dag.Agent()
			err := agent.Run(agent.Context)
			require.NoError(t, err)
			dag.AssertLatestStatus(t, status.Success)

			dagStatus, statusErr := dag.DAGRunMgr.GetLatestStatus(dag.Context, dag.DAG)
			require.NoError(t, statusErr)

			require.Len(t, dagStatus.Nodes, tc.expectedNodes)

			require.Greater(t, len(dagStatus.Nodes), tc.parallelNodeIndex, "node index out of range")
			parallelNode := dagStatus.Nodes[tc.parallelNodeIndex]
			require.Equal(t, status.NodeSuccess, parallelNode.Status)
			require.Len(t, parallelNode.Children, tc.expectedChildren)

			if tc.verify != nil {
				tc.verify(t, &dagStatus, parallelNode)
			}
		})
	}
}

func TestParallelExecution_WithOutput(t *testing.T) {
	const dagContent = `steps:
  - run: child-with-output
    parallel:
      items:
        - "A"
        - "B"
        - "C"
    output: PARALLEL_RESULTS
  - command: |
      echo "Parallel execution results:"
      echo "${PARALLEL_RESULTS}"
    output: FINAL_OUTPUT
---
name: child-with-output
params:
  - ITEM: ""
steps:
  - command: |
      echo "Processing item: $1"
      echo "TASK_RESULT_$1"
    output: TASK_OUTPUT
`

	th := test.Setup(t)
	dag := th.DAG(t, dagContent)
	agent := dag.Agent()
	err := agent.Run(agent.Context)
	require.NoError(t, err)
	dag.AssertLatestStatus(t, status.Success)

	dagStatus, statusErr := dag.DAGRunMgr.GetLatestStatus(dag.Context, dag.DAG)
	require.NoError(t, statusErr)

	require.Len(t, dagStatus.Nodes, 2)

	require.Greater(t, len(dagStatus.Nodes), 0, "node index out of range")
	parallelNode := dagStatus.Nodes[0]
	require.Equal(t, status.NodeSuccess, parallelNode.Status)
	require.Len(t, parallelNode.Children, 3)

	require.NotNil(t, parallelNode.OutputVariables, "no outputs recorded for node %s", parallelNode.Step.Name)
	rawOutput, ok := parallelNode.OutputVariables.Load("PARALLEL_RESULTS")
	require.True(t, ok, "output %q not found", "PARALLEL_RESULTS")
	raw, ok := rawOutput.(string)
	require.True(t, ok, "output %q is not a string", "PARALLEL_RESULTS")
	results := parseParallelResults(t, raw)
	require.Equal(t, 3, results.Summary.Total)
	require.Equal(t, 3, results.Summary.Succeeded)
	require.Equal(t, 0, results.Summary.Failed)

	outputs := collectOutputs(results.Outputs, "TASK_OUTPUT")
	require.Len(t, outputs, 3)
	for _, expected := range []string{"TASK_RESULT_A", "TASK_RESULT_B", "TASK_RESULT_C"} {
		found := false
		for _, out := range outputs {
			if strings.Contains(out, expected) {
				found = true
				break
			}
		}
		require.Truef(t, found, "expected parallel output to contain %s", expected)
	}

	require.Greater(t, len(dagStatus.Nodes), 1, "node index out of range")
	useOutputNode := dagStatus.Nodes[1]
	require.Equal(t, status.NodeSuccess, useOutputNode.Status)
}

func TestParallelExecution_DeterministicIDs(t *testing.T) {
	const dagContent = `steps:
  - run: child-echo
    parallel:
      items:
        - "test1"
        - "test2"
        - "test1"
        - "test3"
        - "test2"
---
name: child-echo
params:
  - ITEM: ""
steps:
  - command: echo "$1"
    output: ECHO_OUTPUT
`

	th := test.Setup(t)
	dag := th.DAG(t, dagContent)
	agent := dag.Agent()
	err := agent.Run(agent.Context)
	require.NoError(t, err)
	dag.AssertLatestStatus(t, status.Success)

	dagStatus, statusErr := dag.DAGRunMgr.GetLatestStatus(dag.Context, dag.DAG)
	require.NoError(t, statusErr)

	require.Len(t, dagStatus.Nodes, 1)
	require.Greater(t, len(dagStatus.Nodes), 0, "node index out of range")
	node := dagStatus.Nodes[0]
	require.Equal(t, status.NodeSuccess, node.Status)
	require.Len(t, node.Children, 3)

	unique := make(map[string]string)
	for _, child := range node.Children {
		unique[child.Params] = child.DAGRunID
	}

	require.Len(t, unique, 3)
	require.Contains(t, unique, "test1")
	require.Contains(t, unique, "test2")
	require.Contains(t, unique, "test3")
}

func TestParallelExecution_PartialFailure(t *testing.T) {
	const dagContent = `steps:
  - run: child-conditional-fail
    parallel:
      items:
        - "ok1"
        - "fail"
        - "ok2"
        - "fail"
        - "ok3"
---
name: child-conditional-fail
params:
  - INPUT: "default"
steps:
  - command: |
      if [ "$1" = "fail" ]; then
        echo "Failing as requested"
        exit 1
      fi
      echo "Processing: $1"
`

	th := test.Setup(t)
	dag := th.DAG(t, dagContent)
	agent := dag.Agent()
	err := agent.Run(agent.Context)
	require.Error(t, err)

	dagStatus, statusErr := dag.DAGRunMgr.GetLatestStatus(dag.Context, dag.DAG)
	require.NoError(t, statusErr)

	require.Len(t, dagStatus.Nodes, 1)
	require.Greater(t, len(dagStatus.Nodes), 0, "node index out of range")
	node := dagStatus.Nodes[0]
	require.Equal(t, status.NodeError, node.Status)
	require.Len(t, node.Children, 4)
}

func TestParallelExecution_OutputCaptureWithFailures(t *testing.T) {
	const dagContent = `steps:
  - run: child-output-fail
    parallel:
      items:
        - "success"
        - "fail"
    output: RESULTS
    continueOn:
      failure: true
---
name: child-output-fail
steps:
  - command: |
      INPUT="$1"
      echo "Output for ${INPUT}"
      if [ "${INPUT}" = "fail" ]; then
        exit 1
      fi
    output: RESULT
`

	th := test.Setup(t)
	dag := th.DAG(t, dagContent)
	agent := dag.Agent()
	err := agent.Run(agent.Context)
	require.Error(t, err)

	dagStatus, statusErr := dag.DAGRunMgr.GetLatestStatus(dag.Context, dag.DAG)
	require.NoError(t, statusErr)

	require.Greater(t, len(dagStatus.Nodes), 0, "node index out of range")
	node := dagStatus.Nodes[0]
	require.Equal(t, status.NodeError, node.Status)

	require.NotNil(t, node.OutputVariables, "no outputs recorded for node %s", node.Step.Name)
	rawOutput, ok := node.OutputVariables.Load("RESULTS")
	require.True(t, ok, "output %q not found", "RESULTS")
	raw, ok := rawOutput.(string)
	require.True(t, ok, "output %q is not a string", "RESULTS")
	results := parseParallelResults(t, raw)
	require.Equal(t, 2, results.Summary.Total)
	require.Equal(t, 1, results.Summary.Succeeded)
	require.Equal(t, 1, results.Summary.Failed)

	outputs := collectOutputs(results.Outputs, "RESULT")
	require.Len(t, outputs, 1)
	require.Contains(t, outputs[0], "Output for success")
	require.NotContains(t, outputs[0], "Output for fail")
}

func TestParallelExecution_OutputCaptureWithRetry(t *testing.T) {
	const counterFile = "/tmp/test_retry_counter.txt"
	t.Cleanup(func() { _ = os.Remove(counterFile) })

	th := test.Setup(t)
	dag := th.DAG(t, fmt.Sprintf(`steps:
  - run: child-retry-simple
    parallel:
      items:
        - "item1"
    output: RESULTS
---
name: child-retry-simple
steps:
  - command: |
      COUNTER_FILE="%s"
      if [ ! -f "$COUNTER_FILE" ]; then
        echo "1" > "$COUNTER_FILE"
        echo "First attempt"
        exit 1
      else
        echo "Retry success"
        exit 0
      fi
    output: OUTPUT
    retryPolicy:
      limit: 1
      intervalSec: 0
`, counterFile))

	agent := dag.Agent()
	err := agent.Run(agent.Context)
	require.NoError(t, err)
	dag.AssertLatestStatus(t, status.Success)

	dagStatus, statusErr := dag.DAGRunMgr.GetLatestStatus(dag.Context, dag.DAG)
	require.NoError(t, statusErr)

	require.Greater(t, len(dagStatus.Nodes), 0, "node index out of range")
	node := dagStatus.Nodes[0]
	require.Equal(t, status.NodeSuccess, node.Status)

	require.NotNil(t, node.OutputVariables, "no outputs recorded for node %s", node.Step.Name)
	rawRaw, ok := node.OutputVariables.Load("RESULTS")
	require.True(t, ok, "output %q not found", "RESULTS")
	raw, ok := rawRaw.(string)
	require.True(t, ok, "output %q is not a string", "RESULTS")
	results := parseParallelResults(t, raw)
	require.Equal(t, 1, results.Summary.Total)
	require.Equal(t, 1, results.Summary.Succeeded)
	require.Equal(t, 0, results.Summary.Failed)

	outputs := collectOutputs(results.Outputs, "OUTPUT")
	require.Len(t, outputs, 1)
	require.Contains(t, outputs[0], "Retry success")
	require.NotContains(t, outputs[0], "First attempt")
	require.NotContains(t, raw, "First attempt")
}

func TestParallelExecution_MinimalRetry(t *testing.T) {
	const dagContent = `steps:
  - run: child-fail
    parallel:
      items:
        - "item1"
    retryPolicy:
      limit: 1
      intervalSec: 1
    output: RESULTS
---
name: child-fail
steps:
  - exit 1
`

	th := test.Setup(t)
	dag := th.DAG(t, dagContent)
	agent := dag.Agent()
	err := agent.Run(agent.Context)
	require.Error(t, err)

	dagStatus, statusErr := dag.DAGRunMgr.GetLatestStatus(dag.Context, dag.DAG)
	require.NoError(t, statusErr)

	require.Greater(t, len(dagStatus.Nodes), 0, "node index out of range")
	node := dagStatus.Nodes[0]
	require.Equal(t, status.NodeError, node.Status)
	require.Equal(t, 1, node.RetryCount)
}

func TestParallelExecution_RetryAndContinueOn(t *testing.T) {
	const dagContent = `steps:
  - run: child-fail-both
    parallel:
      items:
        - "item1"
    retryPolicy:
      limit: 1
      intervalSec: 1
    continueOn:
      failure: true
    output: RESULTS
  - echo "This should run"
---
name: child-fail-both
steps:
  - exit 1
`

	th := test.Setup(t)
	dag := th.DAG(t, dagContent)
	agent := dag.Agent()
	err := agent.Run(agent.Context)
	require.Error(t, err)

	dagStatus, statusErr := dag.DAGRunMgr.GetLatestStatus(dag.Context, dag.DAG)
	require.NoError(t, statusErr)

	require.Len(t, dagStatus.Nodes, 2)

	require.Greater(t, len(dagStatus.Nodes), 0, "node index out of range")
	parallelNode := dagStatus.Nodes[0]
	require.Equal(t, status.NodeError, parallelNode.Status)
	require.Equal(t, 1, parallelNode.RetryCount)

	require.Greater(t, len(dagStatus.Nodes), 1, "node index out of range")
	nextNode := dagStatus.Nodes[1]
	require.Equal(t, status.NodeSuccess, nextNode.Status)

	require.NotNil(t, parallelNode.OutputVariables, "no outputs recorded for node %s", parallelNode.Step.Name)
	_, ok := parallelNode.OutputVariables.Load("RESULTS")
	require.True(t, ok, "output %q not found", "RESULTS")
}

func TestParallelExecution_OutputsArray(t *testing.T) {
	const dagContent = `steps:
  - run: child-with-output
    parallel:
      items: ["task1", "task2", "task3"]
    output: RESULTS
  - command: |
      echo "First output: ${RESULTS.outputs[0].TASK_OUTPUT}"
    output: FIRST_OUTPUT
  - command: |
      echo "Output 0: ${RESULTS.outputs[0].TASK_OUTPUT}"
      echo "Output 1: ${RESULTS.outputs[1].TASK_OUTPUT}"
      echo "Output 2: ${RESULTS.outputs[2].TASK_OUTPUT}"
    output: ALL_OUTPUTS
---
name: child-with-output
params:
  - ITEM: ""
steps:
  - command: |
      echo "Processing item: $1"
      echo "TASK_RESULT_$1"
    output: TASK_OUTPUT
`

	th := test.Setup(t)
	dag := th.DAG(t, dagContent)
	agent := dag.Agent()
	err := agent.Run(agent.Context)
	require.NoError(t, err)
	dag.AssertLatestStatus(t, status.Success)

	dagStatus, statusErr := dag.DAGRunMgr.GetLatestStatus(dag.Context, dag.DAG)
	require.NoError(t, statusErr)

	require.Greater(t, len(dagStatus.Nodes), 0, "node index out of range")
	parallelNode := dagStatus.Nodes[0]
	require.Equal(t, status.NodeSuccess, parallelNode.Status)
	require.Len(t, parallelNode.Children, 3)

	require.Greater(t, len(dagStatus.Nodes), 1, "node index out of range")
	firstNode := dagStatus.Nodes[1]
	require.NotNil(t, firstNode.OutputVariables, "no outputs recorded for node %s", firstNode.Step.Name)
	firstOutputRaw, ok := firstNode.OutputVariables.Load("FIRST_OUTPUT")
	require.True(t, ok, "output %q not found", "FIRST_OUTPUT")
	firstOutput, ok := firstOutputRaw.(string)
	require.True(t, ok, "output %q is not a string", "FIRST_OUTPUT")
	require.Contains(t, firstOutput, "First output:")

	require.Greater(t, len(dagStatus.Nodes), 2, "node index out of range")
	allNode := dagStatus.Nodes[2]
	require.NotNil(t, allNode.OutputVariables, "no outputs recorded for node %s", allNode.Step.Name)
	allOutputsRaw, ok := allNode.OutputVariables.Load("ALL_OUTPUTS")
	require.True(t, ok, "output %q not found", "ALL_OUTPUTS")
	allOutputs, ok := allOutputsRaw.(string)
	require.True(t, ok, "output %q is not a string", "ALL_OUTPUTS")
	require.Contains(t, allOutputs, "TASK_RESULT_task1")
	require.Contains(t, allOutputs, "TASK_RESULT_task2")
	require.Contains(t, allOutputs, "TASK_RESULT_task3")
}

func TestParallelExecution_ExceedsMaxLimit(t *testing.T) {
	items := make([]string, 1001)
	for i := range items {
		items[i] = fmt.Sprintf("        - \"item%d\"", i)
	}

	th := test.Setup(t)
	dag := th.DAG(t, fmt.Sprintf(`steps:
  - run: child-echo
    parallel:
      items:
%s
---
name: child-echo
params:
  - ITEM: ""
steps:
  - command: echo "$1"
    output: ECHO_OUTPUT
`, strings.Join(items, "\n")))

	agent := dag.Agent()
	err := agent.Run(agent.Context)

	_, statusErr := dag.DAGRunMgr.GetLatestStatus(dag.Context, dag.DAG)
	require.NoError(t, statusErr)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parallel execution exceeds maximum limit")
}

func TestParallelExecution_ExactlyMaxLimit(t *testing.T) {
	helper := test.Setup(t)

	items := make([]string, 1000)
	for i := range items {
		items[i] = fmt.Sprintf("        - \"item%d\"", i)
	}

	dag := helper.DAG(t, fmt.Sprintf(`steps:
  - run: child-echo
    parallel:
      items:
%s
      maxConcurrent: 10
---
name: child-echo
params:
  - ITEM: ""
steps:
  - command: echo "$1"
    output: ECHO_OUTPUT
`, strings.Join(items, "\n")))

	agent := dag.Agent()
	errChan := make(chan error, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		agent.Context = ctx
		errChan <- agent.Run(agent.Context)
	}()

	select {
	case err := <-errChan:
		require.NoError(t, err, "DAG should not fail with exactly 1000 items")
	case <-time.After(time.Second):
		cancel()
		<-errChan
	}
}

func TestParallelExecution_ObjectItemProperties(t *testing.T) {
	const dagContent = `steps:
  - command: |
      echo '[
        {"region": "us-east-1", "bucket": "data-us"},
        {"region": "eu-west-1", "bucket": "data-eu"},
        {"region": "ap-south-1", "bucket": "data-ap"}
      ]'
    output: CONFIGS

  - run: sync-data
    parallel:
      items: ${CONFIGS}
      maxConcurrent: 2
    params:
      - REGION: ${ITEM.region}
      - BUCKET: ${ITEM.bucket}
    output: RESULTS

---
name: sync-data
params:
  - REGION: ""
  - BUCKET: ""
steps:
  - script: |
      echo "Syncing data from region: $REGION"
      echo "Using bucket: $BUCKET"
      echo "Sync completed for $BUCKET in $REGION"
    output: SYNC_RESULT
`

	th := test.Setup(t)
	dag := th.DAG(t, dagContent)
	agent := dag.Agent()
	err := agent.Run(agent.Context)
	require.NoError(t, err)
	dag.AssertLatestStatus(t, status.Success)

	dagStatus, statusErr := dag.DAGRunMgr.GetLatestStatus(dag.Context, dag.DAG)
	require.NoError(t, statusErr)

	require.Len(t, dagStatus.Nodes, 2)

	require.Greater(t, len(dagStatus.Nodes), 1, "node index out of range")
	syncNode := dagStatus.Nodes[1]
	require.Equal(t, status.NodeSuccess, syncNode.Status)
	require.Len(t, syncNode.Children, 3)

	require.NotNil(t, syncNode.OutputVariables, "no outputs recorded for node %s", syncNode.Step.Name)
	rawRaw, ok := syncNode.OutputVariables.Load("RESULTS")
	require.True(t, ok, "output %q not found", "RESULTS")
	raw, ok := rawRaw.(string)
	require.True(t, ok, "output %q is not a string", "RESULTS")
	results := parseParallelResults(t, raw)
	require.Equal(t, 3, results.Summary.Total)
	require.Equal(t, 3, results.Summary.Succeeded)
	require.Equal(t, 0, results.Summary.Failed)

	require.Contains(t, raw, "Syncing data from region: us-east-1")
	require.Contains(t, raw, "Using bucket: data-us")
	require.Contains(t, raw, "Sync completed for data-ap in ap-south-1")
}

func TestParallelExecution_DynamicFileDiscovery(t *testing.T) {
	helper := test.Setup(t)

	testDataDir := filepath.Join(helper.Config.Paths.DAGsDir, "test-data")
	require.NoError(t, os.MkdirAll(testDataDir, 0755))
	t.Cleanup(func() { _ = os.RemoveAll(testDataDir) })

	testFiles := []string{"data1.csv", "data2.csv", "data3.csv"}
	for _, file := range testFiles {
		content := fmt.Sprintf("id,name\n1,%s\n", file)
		require.NoError(t, os.WriteFile(filepath.Join(testDataDir, file), []byte(content), 0644))
	}

	helper.CreateDAGFile(t, helper.Config.Paths.DAGsDir, "process-file", []byte(`
params:
  - ITEM: ""
steps:
  - script: |
      FILE="$ITEM"
      echo "Processing file: ${FILE}"
      if [ -f "${FILE}" ]; then
        LINE_COUNT=$(wc -l < "${FILE}")
        echo "File has ${LINE_COUNT} lines"
      else
        echo "ERROR: File not found"
        exit 1
      fi
    output: PROCESS_RESULT
`))

	dag := helper.DAG(t, fmt.Sprintf(`
steps:
  - command: find %s -name "*.csv" -type f
    output: FILES

  - run: process-file
    parallel: ${FILES}
    params:
      - ITEM: ${ITEM}
    output: RESULTS
`, testDataDir))

	agent := dag.Agent()
	err := agent.Run(agent.Context)
	require.NoError(t, err)
	dag.AssertLatestStatus(t, status.Success)

	dagStatus, statusErr := dag.DAGRunMgr.GetLatestStatus(dag.Context, dag.DAG)
	require.NoError(t, statusErr)

	require.Greater(t, len(dagStatus.Nodes), 0, "node index out of range")
	getFiles := dagStatus.Nodes[0]
	require.NotNil(t, getFiles.OutputVariables, "no outputs recorded for node %s", getFiles.Step.Name)
	filesOutputRaw, ok := getFiles.OutputVariables.Load("FILES")
	require.True(t, ok, "output %q not found", "FILES")
	filesOutput, ok := filesOutputRaw.(string)
	require.True(t, ok, "output %q is not a string", "FILES")
	for _, file := range testFiles {
		require.Contains(t, filesOutput, file)
	}

	require.Greater(t, len(dagStatus.Nodes), 1, "node index out of range")
	processFiles := dagStatus.Nodes[1]
	require.Equal(t, status.NodeSuccess, processFiles.Status)
	require.Len(t, processFiles.Children, 3)

	require.NotNil(t, processFiles.OutputVariables, "no outputs recorded for node %s", processFiles.Step.Name)
	rawRaw, ok := processFiles.OutputVariables.Load("RESULTS")
	require.True(t, ok, "output %q not found", "RESULTS")
	raw, ok := rawRaw.(string)
	require.True(t, ok, "output %q is not a string", "RESULTS")
	results := parseParallelResults(t, raw)
	require.Equal(t, 3, results.Summary.Total)
	require.Equal(t, 3, results.Summary.Succeeded)
	require.Equal(t, 0, results.Summary.Failed)

	require.Regexp(t, `File has\s+2 lines`, raw)
	require.Contains(t, raw, "Processing file:")
}

func TestParallelExecution_StaticObjectItems(t *testing.T) {
	const dagContent = `steps:
  - run: deploy-service
    parallel:
      maxConcurrent: 3
      items:
        - name: web-service
          port: 8080
          replicas: 3
        - name: api-service
          port: 8081
          replicas: 2
        - name: worker-service
          port: 8082
          replicas: 5
    params:
      - SERVICE_NAME: ${ITEM.name}
      - PORT: ${ITEM.port}
      - REPLICAS: ${ITEM.replicas}
    continueOn:
      failure: true
    output: DEPLOYMENT_RESULTS
---
name: deploy-service
params:
  - SERVICE_NAME: ""
  - PORT: ""
  - REPLICAS: ""
steps:
  - script: |
      echo "Validating deployment parameters..."
      if [ -z "$SERVICE_NAME" ] || [ -z "$PORT" ] || [ -z "$REPLICAS" ]; then
        echo "ERROR: Missing required parameters"
        exit 1
      fi
      echo "Service: $SERVICE_NAME"
      echo "Port: $PORT"
      echo "Replicas: $REPLICAS"
    output: VALIDATE_RESULT
  - script: |
      echo "Deploying $SERVICE_NAME..."
      echo "  - Binding to port $PORT"
      echo "  - Scaling to $REPLICAS replicas"
      sleep 1
      if [ "$SERVICE_NAME" = "api-service" ]; then
        echo "ERROR: Failed to deploy $SERVICE_NAME - port $PORT already in use"
        exit 1
      fi
      echo "Successfully deployed $SERVICE_NAME"
    output: DEPLOY_RESULT
`

	th := test.Setup(t)
	dag := th.DAG(t, dagContent)
	agent := dag.Agent()
	err := agent.Run(agent.Context)
	require.Error(t, err)

	dagStatus, statusErr := dag.DAGRunMgr.GetLatestStatus(dag.Context, dag.DAG)
	require.NoError(t, statusErr)

	require.Greater(t, len(dagStatus.Nodes), 0, "node index out of range")
	node := dagStatus.Nodes[0]
	require.Equal(t, status.NodeError, node.Status)
	require.Len(t, node.Children, 3)

	require.NotNil(t, node.OutputVariables, "no outputs recorded for node %s", node.Step.Name)
	rawRaw, ok := node.OutputVariables.Load("DEPLOYMENT_RESULTS")
	require.True(t, ok, "output %q not found", "DEPLOYMENT_RESULTS")
	raw, ok := rawRaw.(string)
	require.True(t, ok, "output %q is not a string", "DEPLOYMENT_RESULTS")
	results := parseParallelResults(t, raw)
	require.Equal(t, 3, results.Summary.Total)
	require.Equal(t, 2, results.Summary.Succeeded)
	require.Equal(t, 1, results.Summary.Failed)

	require.Contains(t, raw, "Service: web-service")
	require.Contains(t, raw, "Successfully deployed worker-service")
	require.NotContains(t, raw, "Successfully deployed api-service")
}

// TestIssue1274_ParallelJSONSingleItem tests that parallel execution
// correctly handles a single JSON item from output (should dispatch 1 job)
func TestIssue1274_ParallelJSONSingleItem(t *testing.T) {
	const dagContent = `steps:
  - command: |
      echo '{"file": "params.txt", "config": "env"}'
    output: jsonList

  - run: issue-1274-worker
    parallel:
      items: ${jsonList}
      maxConcurrent: 1
    params:
      aJson: ${ITEM}
    continueOn:
      skipped: true

---
name: issue-1274-worker
params:
  aJson: ""
steps:
  - name: Process JSON item
    command: echo "Processing file=${aJson.file} config=${aJson.config}"
`

	th := test.Setup(t)
	dag := th.DAG(t, dagContent)
	agent := dag.Agent()
	err := agent.Run(agent.Context)
	require.NoError(t, err)
	dag.AssertLatestStatus(t, status.Success)

	dagStatus, statusErr := dag.DAGRunMgr.GetLatestStatus(dag.Context, dag.DAG)
	require.NoError(t, statusErr)

	require.Len(t, dagStatus.Nodes, 2)

	require.Greater(t, len(dagStatus.Nodes), 1, "node index out of range")
	parallelNode := dagStatus.Nodes[1]
	require.Equal(t, status.NodeSuccess, parallelNode.Status)
	require.Len(t, parallelNode.Children, 1, "should dispatch exactly 1 worker instance for 1 JSON item")
}

// TestIssue1274_ParallelJSONMultipleItems tests that parallel execution
// correctly handles multiple JSON items from output (should dispatch N jobs)
func TestIssue1274_ParallelJSONMultipleItems(t *testing.T) {
	const dagContent = `steps:
  - command: |
      echo '{"file": "file1.txt", "config": "prod"}'
      echo '{"file": "file2.txt", "config": "test"}'
      echo '{"file": "file3.txt", "config": "dev"}'
    output: jsonList

  - run: issue-1274-worker-multi
    parallel:
      items: ${jsonList}
      maxConcurrent: 1
    params:
      aJson: ${ITEM}
    continueOn:
      skipped: true

---
name: issue-1274-worker-multi
params:
  aJson: ""
steps:
  - name: Process JSON item
    command: echo "Processing file=${aJson.file} config=${aJson.config}"
`

	th := test.Setup(t)
	dag := th.DAG(t, dagContent)
	agent := dag.Agent()
	err := agent.Run(agent.Context)
	require.NoError(t, err)
	dag.AssertLatestStatus(t, status.Success)

	dagStatus, statusErr := dag.DAGRunMgr.GetLatestStatus(dag.Context, dag.DAG)
	require.NoError(t, statusErr)

	require.Len(t, dagStatus.Nodes, 2)

	require.Greater(t, len(dagStatus.Nodes), 1, "node index out of range")
	parallelNode := dagStatus.Nodes[1]
	require.Equal(t, status.NodeSuccess, parallelNode.Status)
	require.Len(t, parallelNode.Children, 3, "should dispatch exactly 3 worker instances for 3 JSON items")
}

type parallelSummary struct {
	Total     int `json:"total"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

type parallelResultsPayload struct {
	Summary parallelSummary  `json:"summary"`
	Outputs []map[string]any `json:"outputs"`
	Results []map[string]any `json:"results"`
}

func parseParallelResults(t *testing.T, raw string) parallelResultsPayload {
	t.Helper()
	parts := strings.SplitN(raw, "=", 2)
	require.Len(t, parts, 2, "expected key=value output format")

	var payload parallelResultsPayload
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(parts[1])), &payload))
	return payload
}

func collectOutputs(entries []map[string]any, key string) []string {
	var out []string
	for _, entry := range entries {
		if value, ok := entry[key].(string); ok {
			out = append(out, value)
		}
	}
	return out
}
