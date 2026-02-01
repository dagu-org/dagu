package cmd_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// executeCommand runs a cobra command with silenced errors and usage output.
func executeCommand(ctx context.Context, c *cobra.Command, args []string) error {
	c.SetContext(ctx)
	c.SetArgs(args)
	c.SilenceErrors = true
	c.SilenceUsage = true
	return c.Execute()
}

// waitForDAGRunning waits until the DAG is in running state.
func waitForDAGRunning(t *testing.T, th test.Command, dagLocation string) {
	t.Helper()
	require.Eventually(t, func() bool {
		attempts := th.DAGRunStore.RecentAttempts(th.Context, dagLocation, 1)
		if len(attempts) < 1 {
			return false
		}
		status, err := attempts[0].ReadStatus(th.Context)
		if err != nil {
			return false
		}
		return status.Status == core.Running
	}, time.Second*3, time.Millisecond*50)
}

// stopDAGAndWait stops a DAG and waits for the goroutine to complete.
func stopDAGAndWait(t *testing.T, th test.Command, dagLocation string, done <-chan struct{}) {
	t.Helper()
	th.RunCommand(t, cmd.Stop(), test.CmdTest{Args: []string{"stop", dagLocation}})
	<-done
}

func TestStatusCommand(t *testing.T) {
	t.Run("StatusDAGRunning", func(t *testing.T) {
		th := test.SetupCommand(t)
		dagFile := th.DAG(t, `steps:
  - name: "1"
    command: sleep 10
`)
		done := make(chan struct{})
		go func() {
			th.RunCommand(t, cmd.Start(), test.CmdTest{Args: []string{"start", dagFile.Location}})
			close(done)
		}()

		waitForDAGRunning(t, th, dagFile.Location)

		err := executeCommand(th.Context, cmd.Status(), []string{dagFile.Location})
		require.NoError(t, err)

		stopDAGAndWait(t, th, dagFile.Location, done)
	})

	t.Run("StatusDAGSuccess", func(t *testing.T) {
		th := test.SetupCommand(t)
		dagFile := th.DAG(t, `steps:
  - name: "success"
    command: "echo 'Success!'"
`)
		err := executeCommand(th.Context, cmd.Start(), []string{dagFile.Location})
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)

		err = executeCommand(th.Context, cmd.Status(), []string{dagFile.Location})
		require.NoError(t, err)
	})

	t.Run("StatusDAGError", func(t *testing.T) {
		th := test.SetupCommand(t)
		dagFile := th.DAG(t, `steps:
  - name: "error"
    command: exit 1
`)
		dag, err := th.DAGStore.GetMetadata(th.Context, dagFile.Location)
		require.NoError(t, err)

		dagRunID := uuid.Must(uuid.NewV7()).String()
		attempt, err := th.DAGRunStore.CreateAttempt(th.Context, dag, time.Now(), dagRunID, exec.NewDAGRunAttemptOptions{})
		require.NoError(t, err)

		err = attempt.Open(th.Context)
		require.NoError(t, err)

		status := exec.DAGRunStatus{
			Name:       dag.Name,
			DAGRunID:   dagRunID,
			Status:     core.Failed,
			StartedAt:  time.Now().Format(time.RFC3339),
			FinishedAt: time.Now().Format(time.RFC3339),
			AttemptID:  attempt.ID(),
			Nodes: []*exec.Node{
				{
					Step:   core.Step{Name: "error"},
					Status: core.NodeFailed,
					Error:  "exit status 1",
				},
			},
		}
		err = attempt.Write(th.Context, status)
		require.NoError(t, err)

		err = attempt.Close(th.Context)
		require.NoError(t, err)

		err = executeCommand(th.Context, cmd.Status(), []string{dagFile.Location})
		require.NoError(t, err)
	})

	t.Run("StatusDAGWithParams", func(t *testing.T) {
		th := test.SetupCommand(t)
		dagFile := th.DAG(t, `params:
  - param1
  - param2
steps:
  - name: "print-params"
    command: "echo Param1: ${param1}, Param2: ${param2}"
`)
		err := executeCommand(th.Context, cmd.Start(), []string{dagFile.Location, "--params=custom1 custom2"})
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)

		err = executeCommand(th.Context, cmd.Status(), []string{dagFile.Location})
		require.NoError(t, err)
	})

	t.Run("StatusDAGWithSpecificRunID", func(t *testing.T) {
		th := test.SetupCommand(t)
		dagFile := th.DAG(t, `steps:
  - name: "success"
    command: "echo 'Success!'"
`)
		runID := uuid.Must(uuid.NewV7()).String()

		err := executeCommand(th.Context, cmd.Start(), []string{dagFile.Location, "--run-id=" + runID})
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)

		err = executeCommand(th.Context, cmd.Status(), []string{dagFile.Location, "--run-id=" + runID})
		require.NoError(t, err)
	})

	t.Run("StatusDAGMultipleRuns", func(t *testing.T) {
		th := test.SetupCommand(t)
		dagFile := th.DAG(t, `steps:
  - name: "success"
    command: "echo 'Success!'"
`)
		err := executeCommand(th.Context, cmd.Start(), []string{dagFile.Location})
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)

		err = executeCommand(th.Context, cmd.Start(), []string{dagFile.Location})
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)

		err = executeCommand(th.Context, cmd.Status(), []string{dagFile.Location})
		require.NoError(t, err)

		dagFile.AssertDAGRunCount(t, 2)
	})

	t.Run("StatusDAGWithSkippedSteps", func(t *testing.T) {
		th := test.SetupCommand(t)
		dagFile := th.DAG(t, `steps:
  - name: "check"
    command: "false"
    continueOn:
      failure: true
  - name: "skipped"
    command: "echo 'This will be skipped'"
    preconditions:
      - condition: "test -f /nonexistent"
`)
		dag, err := th.DAGStore.GetMetadata(th.Context, dagFile.Location)
		require.NoError(t, err)

		dagRunID := uuid.Must(uuid.NewV7()).String()
		attempt, err := th.DAGRunStore.CreateAttempt(th.Context, dag, time.Now(), dagRunID, exec.NewDAGRunAttemptOptions{})
		require.NoError(t, err)

		err = attempt.Open(th.Context)
		require.NoError(t, err)

		now := time.Now().Format(time.RFC3339)
		status := exec.DAGRunStatus{
			Name:       dag.Name,
			DAGRunID:   dagRunID,
			Status:     core.Failed,
			StartedAt:  now,
			FinishedAt: now,
			AttemptID:  attempt.ID(),
			Nodes: []*exec.Node{
				{
					Step:       core.Step{Name: "check"},
					Status:     core.NodeFailed,
					Error:      "exit status 1",
					StartedAt:  now,
					FinishedAt: now,
				},
				{
					Step:       core.Step{Name: "skipped"},
					Status:     core.NodeSkipped,
					StartedAt:  "-",
					FinishedAt: now,
				},
			},
		}
		err = attempt.Write(th.Context, status)
		require.NoError(t, err)

		err = attempt.Close(th.Context)
		require.NoError(t, err)

		err = executeCommand(th.Context, cmd.Status(), []string{dagFile.Location})
		require.NoError(t, err)
	})

	t.Run("StatusDAGCancel", func(t *testing.T) {
		th := test.SetupCommand(t)
		dagFile := th.DAG(t, `steps:
  - name: "1"
    command: sleep 10
`)
		done := make(chan struct{})
		go func() {
			th.RunCommand(t, cmd.Start(), test.CmdTest{Args: []string{"start", dagFile.Location}})
			close(done)
		}()

		waitForDAGRunning(t, th, dagFile.Location)

		th.RunCommand(t, cmd.Stop(), test.CmdTest{Args: []string{"stop", dagFile.Location}})
		<-done

		err := executeCommand(th.Context, cmd.Status(), []string{dagFile.Location})
		require.NoError(t, err)
	})

	t.Run("StatusDAGWithManySteps", func(t *testing.T) {
		th := test.SetupCommand(t)
		dagFile := th.DAG(t, `steps:
  - name: "step1"
    command: "echo 'Step 1'"
  - name: "step2"
    command: "echo 'Step 2'"
  - name: "step3"
    command: "echo 'Step 3'"
  - name: "step4"
    command: "echo 'Step 4'"
  - name: "step5"
    command: "echo 'Step 5'"
  - name: "step6"
    command: "echo 'Step 6'"
  - name: "step7"
    command: "echo 'Step 7'"
  - name: "step8"
    command: "echo 'Step 8'"
  - name: "step9"
    command: "echo 'Step 9'"
  - name: "step10"
    command: "echo 'Step 10'"
`)
		err := executeCommand(th.Context, cmd.Start(), []string{dagFile.Location})
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)

		err = executeCommand(th.Context, cmd.Status(), []string{dagFile.Location})
		require.NoError(t, err)
	})

	t.Run("StatusDAGByName", func(t *testing.T) {
		th := test.SetupCommand(t)
		dagFile := th.DAG(t, `steps:
  - name: "success"
    command: "echo 'Success!'"
`)
		err := executeCommand(th.Context, cmd.Start(), []string{dagFile.Location})
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)

		err = executeCommand(th.Context, cmd.Status(), []string{dagFile.Location})
		require.NoError(t, err)
	})

	t.Run("StatusDAGWithPID", func(t *testing.T) {
		th := test.SetupCommand(t)
		dagFile := th.DAG(t, `steps:
  - name: "1"
    command: sleep 10
`)
		done := make(chan struct{})
		go func() {
			th.RunCommand(t, cmd.Start(), test.CmdTest{Args: []string{"start", dagFile.Location}})
			close(done)
		}()

		waitForDAGRunning(t, th, dagFile.Location)

		err := executeCommand(th.Context, cmd.Status(), []string{dagFile.Location})
		require.NoError(t, err)

		stopDAGAndWait(t, th, dagFile.Location, done)
	})

	t.Run("StatusDAGWithAttemptID", func(t *testing.T) {
		th := test.SetupCommand(t)
		dagFile := th.DAG(t, `steps:
  - name: "success"
    command: "echo 'Success!'"
`)
		err := executeCommand(th.Context, cmd.Start(), []string{dagFile.Location})
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)

		ctx := context.Background()
		dag, err := th.DAGStore.GetMetadata(ctx, dagFile.Location)
		require.NoError(t, err)

		status, err := th.DAGRunMgr.GetLatestStatus(ctx, dag)
		require.NoError(t, err)
		require.NotEmpty(t, status.AttemptID)

		err = executeCommand(th.Context, cmd.Status(), []string{dagFile.Location})
		require.NoError(t, err)
	})

	t.Run("StatusDAGWithLogPaths", func(t *testing.T) {
		th := test.SetupCommand(t)
		dagFile := th.DAG(t, `steps:
  - name: "success"
    command: "echo 'Success!'"
`)
		err := executeCommand(th.Context, cmd.Start(), []string{dagFile.Location})
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)

		err = executeCommand(th.Context, cmd.Status(), []string{dagFile.Location})
		require.NoError(t, err)
	})

	t.Run("StatusDAGWithBinaryLogContent", func(t *testing.T) {
		th := test.SetupCommand(t)
		dagFile := th.DAG(t, `steps:
  - name: "success"
    command: "echo 'Success!'"
`)
		dag, err := th.DAGStore.GetMetadata(th.Context, dagFile.Location)
		require.NoError(t, err)

		dagRunID := uuid.Must(uuid.NewV7()).String()
		attempt, err := th.DAGRunStore.CreateAttempt(th.Context, dag, time.Now(), dagRunID, exec.NewDAGRunAttemptOptions{})
		require.NoError(t, err)

		err = attempt.Open(th.Context)
		require.NoError(t, err)

		now := time.Now().Format(time.RFC3339)
		status := exec.DAGRunStatus{
			Name:       dag.Name,
			DAGRunID:   dagRunID,
			Status:     core.Succeeded,
			StartedAt:  now,
			FinishedAt: now,
			AttemptID:  attempt.ID(),
			Nodes: []*exec.Node{
				{
					Step:   core.Step{Name: "binary_output"},
					Status: core.NodeSucceeded,
					Stdout: "/nonexistent/binary.log",
					Stderr: "",
				},
			},
		}
		err = attempt.Write(th.Context, status)
		require.NoError(t, err)

		err = attempt.Close(th.Context)
		require.NoError(t, err)

		err = executeCommand(th.Context, cmd.Status(), []string{dagFile.Location})
		require.NoError(t, err)
	})

	t.Run("StatusSubDAGRun", func(t *testing.T) {
		th := test.SetupCommand(t)
		dagFile := th.DAG(t, `steps:
  - name: run-child
    call: child-dag
    params: "NAME=World"

---

name: child-dag
params:
  - NAME
steps:
  - name: greet
    command: echo "Hello, ${NAME}!"
`)
		parentRunID := uuid.Must(uuid.NewV7()).String()

		err := executeCommand(th.Context, cmd.Start(), []string{dagFile.Location, "--run-id=" + parentRunID})
		require.NoError(t, err)

		parentRef := exec.NewDAGRunRef(dagFile.Location, parentRunID)
		var parentAttempt exec.DAGRunAttempt
		require.Eventually(t, func() bool {
			var err error
			parentAttempt, err = th.DAGRunStore.FindAttempt(th.Context, parentRef)
			if err != nil {
				return false
			}
			parentStatus, err := parentAttempt.ReadStatus(th.Context)
			if err != nil {
				return false
			}
			return len(parentStatus.Nodes) > 0 && len(parentStatus.Nodes[0].SubRuns) > 0
		}, 5*time.Second, 50*time.Millisecond, "parent status should have nodes with sub-runs")

		parentStatus, err := parentAttempt.ReadStatus(th.Context)
		require.NoError(t, err)
		require.Len(t, parentStatus.Nodes, 1)
		require.NotEmpty(t, parentStatus.Nodes[0].SubRuns)

		subDAGRunID := parentStatus.Nodes[0].SubRuns[0].DAGRunID

		err = executeCommand(th.Context, cmd.Status(), []string{
			dagFile.Location,
			"--run-id=" + parentRunID,
			"--sub-run-id=" + subDAGRunID,
		})
		require.NoError(t, err)
	})

	t.Run("StatusSubDAGRunMissingParentRunID", func(t *testing.T) {
		th := test.SetupCommand(t)
		dagFile := th.DAG(t, `steps:
  - name: "success"
    command: "echo 'Success!'"
`)
		err := executeCommand(th.Context, cmd.Status(), []string{dagFile.Location, "--sub-run-id=some-sub-id"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "--sub-run-id requires --run-id")
	})

	t.Run("StatusSubDAGRunNotFound", func(t *testing.T) {
		th := test.SetupCommand(t)
		dagFile := th.DAG(t, `steps:
  - name: "success"
    command: "echo 'Success!'"
`)
		parentRunID := uuid.Must(uuid.NewV7()).String()

		err := executeCommand(th.Context, cmd.Start(), []string{dagFile.Location, "--run-id=" + parentRunID})
		require.NoError(t, err)

		parentRef := exec.NewDAGRunRef(dagFile.Location, parentRunID)
		require.Eventually(t, func() bool {
			attempt, err := th.DAGRunStore.FindAttempt(th.Context, parentRef)
			if err != nil {
				return false
			}
			status, err := attempt.ReadStatus(th.Context)
			if err != nil {
				return false
			}
			return status.Status != core.Running
		}, 5*time.Second, 50*time.Millisecond, "DAG run should complete")

		err = executeCommand(th.Context, cmd.Status(), []string{
			dagFile.Location,
			"--run-id=" + parentRunID,
			"--sub-run-id=non-existent-sub-id",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to find sub dag-run")
	})
}
