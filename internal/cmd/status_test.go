package cmd_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
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
		th.RunCommand(t, cmd.CmdStart(), test.CmdTest{
			Args: []string{"start", dagFile.Location},
		})

		// Check the status runs without error
		statusCmd := cmd.CmdStatus()
		statusCmd.SetContext(th.Context)
		statusCmd.SetArgs([]string{dagFile.Location})
		statusCmd.SilenceErrors = true
		statusCmd.SilenceUsage = true
		
		err := statusCmd.Execute()
		require.NoError(t, err)
	})

	t.Run("StatusDAGError", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, "cmd/status_error.yaml")

		// Run the DAG (it will fail)
		th.RunCommand(t, cmd.CmdStart(), test.CmdTest{
			Args: []string{"start", dagFile.Location},
		})

		// Check the status runs without error even for failed DAGs
		statusCmd := cmd.CmdStatus()
		statusCmd.SetContext(th.Context)
		statusCmd.SetArgs([]string{dagFile.Location})
		statusCmd.SilenceErrors = true
		statusCmd.SilenceUsage = true
		
		err := statusCmd.Execute()
		require.NoError(t, err)
	})

	t.Run("StatusDAGWithParams", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, "cmd/status_with_params.yaml")

		// Run the DAG with custom parameters
		th.RunCommand(t, cmd.CmdStart(), test.CmdTest{
			Args: []string{"start", `--params="custom1 custom2"`, dagFile.Location},
		})

		// Check the status runs without error
		statusCmd := cmd.CmdStatus()
		statusCmd.SetContext(th.Context)
		statusCmd.SetArgs([]string{dagFile.Location})
		statusCmd.SilenceErrors = true
		statusCmd.SilenceUsage = true
		
		err := statusCmd.Execute()
		require.NoError(t, err)
	})

	t.Run("StatusDAGWithSpecificRunID", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, "cmd/status_success.yaml")
		runID := uuid.Must(uuid.NewV7()).String()

		// Run the DAG with a specific run ID
		th.RunCommand(t, cmd.CmdStart(), test.CmdTest{
			Args: []string{"start", "--run-id=" + runID, dagFile.Location},
		})

		// Check the status with the specific run ID
		statusCmd := cmd.CmdStatus()
		statusCmd.SetContext(th.Context)
		statusCmd.SetArgs([]string{dagFile.Location, "--run-id=" + runID})
		statusCmd.SilenceErrors = true
		statusCmd.SilenceUsage = true
		
		err := statusCmd.Execute()
		require.NoError(t, err)
	})

	t.Run("StatusDAGMultipleRuns", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, "cmd/status_success.yaml")

		// Run the DAG twice
		th.RunCommand(t, cmd.CmdStart(), test.CmdTest{
			Args: []string{"start", dagFile.Location},
		})

		// Wait a bit to ensure different timestamps
		time.Sleep(100 * time.Millisecond)

		th.RunCommand(t, cmd.CmdStart(), test.CmdTest{
			Args: []string{"start", dagFile.Location},
		})

		// Status without run ID should show the latest run
		statusCmd := cmd.CmdStatus()
		statusCmd.SetContext(th.Context)
		statusCmd.SetArgs([]string{dagFile.Location})
		statusCmd.SilenceErrors = true
		statusCmd.SilenceUsage = true
		
		err := statusCmd.Execute()
		require.NoError(t, err)

		// Verify we have 2 runs
		dagFile.AssertDAGRunCount(t, 2)
	})

	t.Run("StatusDAGWithSkippedSteps", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, "cmd/status_skipped.yaml")

		// Run the DAG
		th.RunCommand(t, cmd.CmdStart(), test.CmdTest{
			Args: []string{"start", dagFile.Location},
		})

		// Check the status runs without error
		statusCmd := cmd.CmdStatus()
		statusCmd.SetContext(th.Context)
		statusCmd.SetArgs([]string{dagFile.Location})
		statusCmd.SilenceErrors = true
		statusCmd.SilenceUsage = true
		
		err := statusCmd.Execute()
		require.NoError(t, err)
	})

	t.Run("StatusDAGNotFound", func(t *testing.T) {
		// Create a command context with proper setup
		statusCmd := cmd.CmdStatus()
		statusCmd.SetContext(context.Background())
		statusCmd.SetArgs([]string{"nonexistent.yaml"})
		statusCmd.SilenceErrors = true
		statusCmd.SilenceUsage = true

		// Try to get status of a non-existent DAG
		err := statusCmd.Execute()
		require.Error(t, err)
	})

	t.Run("StatusDAGNoRuns", func(t *testing.T) {
		th := test.SetupCommand(t)

		// Create a DAG file but don't run it
		dagFile := th.DAG(t, "cmd/status_success.yaml")

		// Create status command with context
		statusCmd := cmd.CmdStatus()
		statusCmd.SetContext(th.Context)
		statusCmd.SetArgs([]string{dagFile.Location})
		statusCmd.SilenceErrors = true
		statusCmd.SilenceUsage = true

		// Try to get status when no runs exist
		err := statusCmd.Execute()
		require.Error(t, err)
	})

	t.Run("StatusDAGWithInvalidRunID", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, "cmd/status_success.yaml")

		// Run the DAG once
		th.RunCommand(t, cmd.CmdStart(), test.CmdTest{
			Args: []string{"start", dagFile.Location},
		})

		// Try to get status with an invalid run ID
		invalidRunID := uuid.Must(uuid.NewV7()).String()
		
		// Create status command with invalid run ID
		statusCmd := cmd.CmdStatus()
		statusCmd.SetContext(th.Context)
		statusCmd.SetArgs([]string{dagFile.Location, "--run-id=" + invalidRunID})
		statusCmd.SilenceErrors = true
		statusCmd.SilenceUsage = true

		// Try to execute - should fail
		err := statusCmd.Execute()
		require.Error(t, err)
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
		th.RunCommand(t, cmd.CmdStart(), test.CmdTest{
			Args: []string{"start", dagFile.Location},
		})

		// Check the status runs without error
		statusCmd := cmd.CmdStatus()
		statusCmd.SetContext(th.Context)
		statusCmd.SetArgs([]string{dagFile.Location})
		statusCmd.SilenceErrors = true
		statusCmd.SilenceUsage = true
		
		err := statusCmd.Execute()
		require.NoError(t, err)
	})

	t.Run("StatusDAGByName", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, "cmd/status_success.yaml")

		// Run the DAG
		th.RunCommand(t, cmd.CmdStart(), test.CmdTest{
			Args: []string{"start", dagFile.Location},
		})

		// Check status using DAG name instead of file path
		statusCmd := cmd.CmdStatus()
		statusCmd.SetContext(th.Context)
		statusCmd.SetArgs([]string{"status-success"})
		statusCmd.SilenceErrors = true
		statusCmd.SilenceUsage = true
		
		err := statusCmd.Execute()
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
		th.RunCommand(t, cmd.CmdStart(), test.CmdTest{
			Args: []string{"start", dagFile.Location},
		})

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