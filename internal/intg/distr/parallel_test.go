package distr_test

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/require"
)

func TestParallel_MultipleItems(t *testing.T) {
	t.Run("parallelExecutionOnWorkers", func(t *testing.T) {
		f := newTestFixture(t, `
steps:
  - name: process-items
    call: child-worker
    parallel:
      items:
        - "item1"
        - "item2"
        - "item3"
      max_concurrent: 2
    output: RESULTS

---
name: child-worker
worker_selector:
  type: test-worker
steps:
  - name: process
    command: echo "Processing $1 on worker"
    output: RESULT
`, withWorkerCount(2), withLabels(map[string]string{"type": "test-worker"}), withLogPersistence())

		agent := f.dagWrapper.Agent()
		agent.RunSuccess(t)
		f.dagWrapper.AssertLatestStatus(t, core.Succeeded)

		st, err := f.latestStatus()
		require.NoError(t, err)
		require.NotNil(t, st)
		require.Len(t, st.Nodes, 1)

		processNode := st.Nodes[0]
		require.Equal(t, "process-items", processNode.Step.Name)
		require.Equal(t, core.NodeSucceeded, processNode.Status)

		require.NotEmpty(t, processNode.SubRuns)
		require.Len(t, processNode.SubRuns, 3)

		for _, child := range processNode.SubRuns {
			require.Contains(t, child.Params, "item")
		}

		require.NotNil(t, processNode.OutputVariables)
		if value, ok := processNode.OutputVariables.Load("RESULTS"); ok {
			results := value.(string)
			require.Contains(t, results, "RESULTS=")
			require.Contains(t, results, `"total": 3`)
			require.Contains(t, results, `"succeeded": 3`)
			require.Contains(t, results, `"failed": 0`)

			require.Contains(t, results, "Processing item1 on worker")
			require.Contains(t, results, "Processing item2 on worker")
			require.Contains(t, results, "Processing item3 on worker")
		} else {
			t.Fatal("RESULTS output not found")
		}
	})
}

func TestParallel_SameWorkerType(t *testing.T) {
	t.Run("allItemsGoToSameWorkerType", func(t *testing.T) {
		f := newTestFixture(t, `
steps:
  - name: process-regions
    call: child-regional
    parallel:
      items:
        - "us-east"
        - "eu-west"
        - "ap-south"
    output: RESULTS

---
name: child-regional
worker_selector:
  type: test-worker
steps:
  - name: process
    command: |
      echo "Processing region: $1"
    output: RESULT
`, withWorkerCount(3), withLabels(map[string]string{"type": "test-worker"}), withLogPersistence())

		agent := f.dagWrapper.Agent()
		agent.RunSuccess(t)
		f.dagWrapper.AssertLatestStatus(t, core.Succeeded)

		st, err := f.latestStatus()
		require.NoError(t, err)
		require.NotNil(t, st)

		processNode := st.Nodes[0]
		require.Equal(t, "process-regions", processNode.Step.Name)
		require.Equal(t, core.NodeSucceeded, processNode.Status)
		require.Len(t, processNode.SubRuns, 3)

		if value, ok := processNode.OutputVariables.Load("RESULTS"); ok {
			results := value.(string)
			require.Contains(t, results, "Processing region: us-east")
			require.Contains(t, results, "Processing region: eu-west")
			require.Contains(t, results, "Processing region: ap-south")
			require.Contains(t, results, `"succeeded": 3`)
		} else {
			t.Fatal("RESULTS output not found")
		}
	})
}

func TestParallel_PartialFailure(t *testing.T) {
	t.Run("partialFailurePropagatesToParentStep", func(t *testing.T) {
		f := newTestFixture(t, `
steps:
  - name: process-items
    call: child-worker
    parallel:
      items:
        - "ok"
        - "fail"

---
name: child-worker
worker_selector:
  type: test-worker
steps:
  - name: run
    command: |
      if [ "$1" = "fail" ]; then
        echo "Simulated failure"
        exit 1
      fi
      echo "Processed $1"
`, withLabels(map[string]string{"type": "test-worker"}), withLogPersistence())

		agent := f.dagWrapper.Agent()
		err := agent.Run(agent.Context)
		require.Error(t, err)

		st, statusErr := f.latestStatus()
		require.NoError(t, statusErr)
		require.NotNil(t, st)
		require.Len(t, st.Nodes, 1)

		node := st.Nodes[0]
		require.Equal(t, "process-items", node.Step.Name)
		require.Equal(t, core.NodeFailed, node.Status)
		require.Len(t, node.SubRuns, 2)
	})
}

func TestParallel_NoMatchingWorkers(t *testing.T) {
	t.Run("failsGracefullyWhenNoWorkersMatch", func(t *testing.T) {
		f := newTestFixture(t, `
steps:
  - name: process-items
    call: child-nonexistent
    parallel:
      items: ["a", "b", "c"]
    output: RESULTS

---
name: child-nonexistent
worker_selector:
  type: nonexistent-worker
steps:
  - name: process
    command: echo "Should not run"
`, withWorkerCount(0))

		agent := f.dagWrapper.Agent()

		ctx, cancel := context.WithTimeout(f.coord.Context, 5*time.Second)
		defer cancel()
		err := agent.Run(ctx)
		require.Error(t, err)

		st := agent.Status(f.coord.Context)
		require.NotEqual(t, core.Succeeded, st.Status)
	})
}

func TestParallel_MixedLocalAndDistributed(t *testing.T) {
	t.Run("mixedLocalAndDistributedExecution", func(t *testing.T) {
		tmpDir := t.TempDir()
		f := newTestFixture(t, `
type: graph
steps:
  - name: local-execution
    call: child-local
    parallel:
      items: ["3", "5"]
    output: LOCAL_RESULTS
    depends: []
  - name: distributed-execution
    call: child-distributed
    parallel:
      items: ["4", "6"]
    output: DISTRIBUTED_RESULTS
    depends: []

---
name: child-local
steps:
  - name: sleep
    command: sleep $1

---
name: child-distributed
worker_selector:
  type: test-worker
steps:
  - name: sleep
    command: sleep $1
`, withLabels(map[string]string{"type": "test-worker"}), withDAGsDir(tmpDir), withLogPersistence())

		agent := f.dagWrapper.Agent()
		done := make(chan struct{})

		go func() {
			agent.Context = f.coord.Context
			_ = agent.Run(agent.Context)
			close(done)
		}()

		require.Eventually(t, func() bool {
			st, err := f.latestStatus()
			if err != nil || !st.Status.IsActive() {
				return false
			}
			if len(st.Nodes) == 0 {
				return false
			}
			var started int
			for _, node := range st.Nodes {
				if node.Status == core.NodeRunning {
					started++
				}
			}
			return started == 2
		}, 5*time.Second, 100*time.Millisecond)

		agent.Signal(f.coord.Context, os.Signal(syscall.SIGTERM))

		<-done

		st := agent.Status(f.coord.Context)

		for _, node := range st.Nodes {
			if node.Step.Name == "local-execution" || node.Step.Name == "distributed-execution" {
				require.Equal(t, core.NodeAborted, node.Status,
					"node %s should be canceled, got %v", node.Step.Name, node.Status)
			}
		}
	})
}
