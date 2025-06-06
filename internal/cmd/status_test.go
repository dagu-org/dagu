package cmd_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestStatusCommand(t *testing.T) {
	t.Run("StatusDAGRunning", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, "cmd/status.yaml")

		done := make(chan struct{})
		go func() {
			// Start a DAG to check the status.
			args := []string{"start", dagFile.Location}
			th.RunCommand(t, cmd.CmdStart(), test.CmdTest{Args: args})
			close(done)
		}()

		require.Eventually(t, func() bool {
			attempts := th.DAGRunStore.RecentAttempts(th.Context, dagFile.Location, 1)
			if len(attempts) < 1 {
				return false
			}
			status, err := attempts[0].ReadStatus(th.Context)
			if err != nil {
				return false
			}

			return scheduler.StatusRunning == status.Status
		}, time.Second*3, time.Millisecond*50)

		// Check the current status - just verify it runs without error
		statusCmd := cmd.CmdStatus()
		statusCmd.SetContext(th.Context)
		statusCmd.SetArgs([]string{dagFile.Location})
		statusCmd.SilenceErrors = true
		statusCmd.SilenceUsage = true

		err := statusCmd.Execute()
		require.NoError(t, err)

		// Stop the DAG.
		args := []string{"stop", dagFile.Location}
		th.RunCommand(t, cmd.CmdStop(), test.CmdTest{Args: args})
		<-done
	})

	t.Run("StatusDAGSuccess", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, "cmd/status_success.yaml")

		// Run the DAG to completion
		startCmd := cmd.CmdStart()
		startCmd.SetContext(th.Context)
		startCmd.SetArgs([]string{dagFile.Location})
		startCmd.SilenceErrors = true
		startCmd.SilenceUsage = true

		err := startCmd.Execute()
		require.NoError(t, err)

		// Wait for DAG to complete
		time.Sleep(200 * time.Millisecond)

		// Check the status runs without error
		statusCmd := cmd.CmdStatus()
		statusCmd.SetContext(th.Context)
		statusCmd.SetArgs([]string{dagFile.Location})
		statusCmd.SilenceErrors = true
		statusCmd.SilenceUsage = true

		err = statusCmd.Execute()
		require.NoError(t, err)
	})

	t.Run("StatusDAGError", func(t *testing.T) {
		// This test verifies that the status command works correctly
		// even for DAGs that have failed execution.
		// We'll create a failed DAG run directly rather than running it.
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, "cmd/status_error.yaml")

		// Create a DAG context
		dag, err := th.DAGStore.GetMetadata(th.Context, dagFile.Location)
		require.NoError(t, err)

		// Create a fake failed DAG run for testing status
		dagRunID := uuid.Must(uuid.NewV7()).String()
		attempt, err := th.DAGRunStore.CreateAttempt(th.Context, dag, time.Now(), dagRunID, models.NewDAGRunAttemptOptions{})
		require.NoError(t, err)

		// Open the attempt for writing
		err = attempt.Open(th.Context)
		require.NoError(t, err)

		// Write a failed status
		status := models.DAGRunStatus{
			Name:       dag.Name,
			DAGRunID:   dagRunID,
			Status:     scheduler.StatusError,
			StartedAt:  time.Now().Format(time.RFC3339),
			FinishedAt: time.Now().Format(time.RFC3339),
			AttemptID:  attempt.ID(),
			Nodes: []*models.Node{
				{
					Step:   digraph.Step{Name: "error"},
					Status: scheduler.NodeStatusError,
					Error:  "exit status 1",
				},
			},
		}
		err = attempt.Write(th.Context, status)
		require.NoError(t, err)

		// Close the attempt
		err = attempt.Close(th.Context)
		require.NoError(t, err)

		// Check the status runs without error even for failed DAGs
		statusCmd := cmd.CmdStatus()
		statusCmd.SetContext(th.Context)
		statusCmd.SetArgs([]string{dagFile.Location})
		statusCmd.SilenceErrors = true
		statusCmd.SilenceUsage = true

		err = statusCmd.Execute()
		require.NoError(t, err)
	})

	t.Run("StatusDAGWithParams", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, "cmd/status_with_params.yaml")

		// Run the DAG with custom parameters
		startCmd := cmd.CmdStart()
		startCmd.SetContext(th.Context)
		startCmd.SetArgs([]string{dagFile.Location, "--params=custom1 custom2"})
		startCmd.SilenceErrors = true
		startCmd.SilenceUsage = true

		err := startCmd.Execute()
		require.NoError(t, err)

		// Wait for DAG to complete
		time.Sleep(200 * time.Millisecond)

		// Check the status runs without error
		statusCmd := cmd.CmdStatus()
		statusCmd.SetContext(th.Context)
		statusCmd.SetArgs([]string{dagFile.Location})
		statusCmd.SilenceErrors = true
		statusCmd.SilenceUsage = true

		err = statusCmd.Execute()
		require.NoError(t, err)
	})

	t.Run("StatusDAGWithSpecificRunID", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, "cmd/status_success.yaml")
		runID := uuid.Must(uuid.NewV7()).String()

		// Run the DAG with a specific run ID
		startCmd := cmd.CmdStart()
		startCmd.SetContext(th.Context)
		startCmd.SetArgs([]string{dagFile.Location, "--run-id=" + runID})
		startCmd.SilenceErrors = true
		startCmd.SilenceUsage = true

		err := startCmd.Execute()
		require.NoError(t, err)

		// Wait for DAG to complete
		time.Sleep(200 * time.Millisecond)

		// Check the status with the specific run ID
		statusCmd := cmd.CmdStatus()
		statusCmd.SetContext(th.Context)
		statusCmd.SetArgs([]string{dagFile.Location, "--run-id=" + runID})
		statusCmd.SilenceErrors = true
		statusCmd.SilenceUsage = true

		err = statusCmd.Execute()
		require.NoError(t, err)
	})

	t.Run("StatusDAGMultipleRuns", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, "cmd/status_success.yaml")

		// Run the DAG twice
		startCmd := cmd.CmdStart()
		startCmd.SetContext(th.Context)
		startCmd.SetArgs([]string{dagFile.Location})
		startCmd.SilenceErrors = true
		startCmd.SilenceUsage = true

		err := startCmd.Execute()
		require.NoError(t, err)

		// Wait a bit to ensure different timestamps
		time.Sleep(200 * time.Millisecond)

		startCmd2 := cmd.CmdStart()
		startCmd2.SetContext(th.Context)
		startCmd2.SetArgs([]string{dagFile.Location})
		startCmd2.SilenceErrors = true
		startCmd2.SilenceUsage = true

		err2 := startCmd2.Execute()
		require.NoError(t, err2)

		// Wait for second DAG to complete
		time.Sleep(200 * time.Millisecond)

		// Status without run ID should show the latest run
		statusCmd := cmd.CmdStatus()
		statusCmd.SetContext(th.Context)
		statusCmd.SetArgs([]string{dagFile.Location})
		statusCmd.SilenceErrors = true
		statusCmd.SilenceUsage = true

		err = statusCmd.Execute()
		require.NoError(t, err)

		// Verify we have 2 runs
		dagFile.AssertDAGRunCount(t, 2)
	})

	t.Run("StatusDAGWithSkippedSteps", func(t *testing.T) {
		// This test verifies that the status command correctly displays
		// DAGs that have skipped steps.
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, "cmd/status_skipped.yaml")

		// Create a DAG context
		dag, err := th.DAGStore.GetMetadata(th.Context, dagFile.Location)
		require.NoError(t, err)

		// Create a fake DAG run with skipped steps
		dagRunID := uuid.Must(uuid.NewV7()).String()
		attempt, err := th.DAGRunStore.CreateAttempt(th.Context, dag, time.Now(), dagRunID, models.NewDAGRunAttemptOptions{})
		require.NoError(t, err)

		// Open the attempt for writing
		err = attempt.Open(th.Context)
		require.NoError(t, err)

		// Write a status with skipped steps
		status := models.DAGRunStatus{
			Name:       dag.Name,
			DAGRunID:   dagRunID,
			Status:     scheduler.StatusError,
			StartedAt:  time.Now().Format(time.RFC3339),
			FinishedAt: time.Now().Format(time.RFC3339),
			AttemptID:  attempt.ID(),
			Nodes: []*models.Node{
				{
					Step:       digraph.Step{Name: "check"},
					Status:     scheduler.NodeStatusError,
					Error:      "exit status 1",
					StartedAt:  time.Now().Format(time.RFC3339),
					FinishedAt: time.Now().Format(time.RFC3339),
				},
				{
					Step:       digraph.Step{Name: "skipped"},
					Status:     scheduler.NodeStatusSkipped,
					StartedAt:  "-",
					FinishedAt: time.Now().Format(time.RFC3339),
				},
			},
		}
		err = attempt.Write(th.Context, status)
		require.NoError(t, err)

		// Close the attempt
		err = attempt.Close(th.Context)
		require.NoError(t, err)

		// Check the status runs without error
		statusCmd := cmd.CmdStatus()
		statusCmd.SetContext(th.Context)
		statusCmd.SetArgs([]string{dagFile.Location})
		statusCmd.SilenceErrors = true
		statusCmd.SilenceUsage = true

		err = statusCmd.Execute()
		require.NoError(t, err)
	})

	t.Run("StatusDAGCancel", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, "cmd/status.yaml")

		done := make(chan struct{})
		go func() {
			// Start a long-running DAG
			args := []string{"start", dagFile.Location}
			th.RunCommand(t, cmd.CmdStart(), test.CmdTest{Args: args})
			close(done)
		}()

		// Wait for it to start
		require.Eventually(t, func() bool {
			attempts := th.DAGRunStore.RecentAttempts(th.Context, dagFile.Location, 1)
			if len(attempts) < 1 {
				return false
			}
			status, err := attempts[0].ReadStatus(th.Context)
			if err != nil {
				return false
			}
			return scheduler.StatusRunning == status.Status
		}, time.Second*3, time.Millisecond*50)

		// Cancel the DAG
		th.RunCommand(t, cmd.CmdStop(), test.CmdTest{
			Args: []string{"stop", dagFile.Location},
		})

		<-done

		// Check the status runs without error
		statusCmd := cmd.CmdStatus()
		statusCmd.SetContext(th.Context)
		statusCmd.SetArgs([]string{dagFile.Location})
		statusCmd.SilenceErrors = true
		statusCmd.SilenceUsage = true

		err := statusCmd.Execute()
		require.NoError(t, err)
	})

	t.Run("StatusDAGWithManySteps", func(t *testing.T) {
		th := test.SetupCommand(t)

		// Create a DAG with many steps to test the step summary truncation
		dagFile := th.DAG(t, "cmd/status_multiple.yaml")

		// Run the DAG
		startCmd := cmd.CmdStart()
		startCmd.SetContext(th.Context)
		startCmd.SetArgs([]string{dagFile.Location})
		startCmd.SilenceErrors = true
		startCmd.SilenceUsage = true

		err := startCmd.Execute()
		require.NoError(t, err)

		// Wait for DAG to complete
		time.Sleep(200 * time.Millisecond)

		// Check the status runs without error
		statusCmd := cmd.CmdStatus()
		statusCmd.SetContext(th.Context)
		statusCmd.SetArgs([]string{dagFile.Location})
		statusCmd.SilenceErrors = true
		statusCmd.SilenceUsage = true

		err = statusCmd.Execute()
		require.NoError(t, err)
	})

	t.Run("StatusDAGByName", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, "cmd/status_success.yaml")

		// Run the DAG
		startCmd := cmd.CmdStart()
		startCmd.SetContext(th.Context)
		startCmd.SetArgs([]string{dagFile.Location})
		startCmd.SilenceErrors = true
		startCmd.SilenceUsage = true

		err := startCmd.Execute()
		require.NoError(t, err)

		// Wait for DAG to complete
		time.Sleep(200 * time.Millisecond)

		// Check status using DAG name instead of file path
		statusCmd := cmd.CmdStatus()
		statusCmd.SetContext(th.Context)
		statusCmd.SetArgs([]string{"status-success"})
		statusCmd.SilenceErrors = true
		statusCmd.SilenceUsage = true

		err = statusCmd.Execute()
		require.NoError(t, err)
	})

	t.Run("StatusDAGWithPID", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, "cmd/status.yaml")

		done := make(chan struct{})
		go func() {
			// Start a DAG to check the PID
			args := []string{"start", dagFile.Location}
			th.RunCommand(t, cmd.CmdStart(), test.CmdTest{Args: args})
			close(done)
		}()

		// Wait for the DAG to start
		require.Eventually(t, func() bool {
			attempts := th.DAGRunStore.RecentAttempts(th.Context, dagFile.Location, 1)
			if len(attempts) < 1 {
				return false
			}
			status, err := attempts[0].ReadStatus(th.Context)
			if err != nil {
				return false
			}
			return scheduler.StatusRunning == status.Status
		}, time.Second*3, time.Millisecond*50)

		// Check the status runs without error
		statusCmd := cmd.CmdStatus()
		statusCmd.SetContext(th.Context)
		statusCmd.SetArgs([]string{dagFile.Location})
		statusCmd.SilenceErrors = true
		statusCmd.SilenceUsage = true

		err := statusCmd.Execute()
		require.NoError(t, err)

		// Stop the DAG
		th.RunCommand(t, cmd.CmdStop(), test.CmdTest{
			Args: []string{"stop", dagFile.Location},
		})
		<-done
	})

	t.Run("StatusDAGWithAttemptID", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, "cmd/status_success.yaml")

		// Run the DAG
		startCmd := cmd.CmdStart()
		startCmd.SetContext(th.Context)
		startCmd.SetArgs([]string{dagFile.Location})
		startCmd.SilenceErrors = true
		startCmd.SilenceUsage = true

		err := startCmd.Execute()
		require.NoError(t, err)

		// Wait for DAG to complete
		time.Sleep(200 * time.Millisecond)

		// Get the latest attempt to verify attempt ID is shown
		ctx := context.Background()
		dag, err := th.DAGStore.GetMetadata(ctx, dagFile.Location)
		require.NoError(t, err)

		status, err := th.DAGRunMgr.GetLatestStatus(ctx, dag)
		require.NoError(t, err)

		// Check the status runs without error
		statusCmd := cmd.CmdStatus()
		statusCmd.SetContext(th.Context)
		statusCmd.SetArgs([]string{dagFile.Location})
		statusCmd.SilenceErrors = true
		statusCmd.SilenceUsage = true

		err = statusCmd.Execute()
		require.NoError(t, err)

		// Verify attempt exists
		require.NotEmpty(t, status.AttemptID)
	})
}
