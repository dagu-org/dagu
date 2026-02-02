package intg_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistoryCommand_Basic(t *testing.T) {
	th := test.SetupCommand(t)
	ctx := context.Background()

	// Create a test DAG
	dag := th.DAG(t, `name: test-history-basic
steps:
  - name: simple-step
    command: "echo test"
`)

	// Execute DAG
	th.RunCommand(t, cmd.Start(), test.CmdTest{
		Args: []string{"start", dag.Location},
	})

	// Wait for completion
	require.Eventually(t, func() bool {
		status, err := th.DAGRunMgr.GetLatestStatus(ctx, dag.DAG)
		return err == nil && status.Status == core.Succeeded
	}, 5*time.Second, 100*time.Millisecond)

	// Run history command
	th.RunCommand(t, cmd.History(), test.CmdTest{
		Name:        "BasicHistory",
		Args:        []string{"history"},
		ExpectedOut: []string{"test-history-basic", "DAG NAME", "RUN ID", "STATUS"},
	})
}

func TestHistoryCommand_FilterByName(t *testing.T) {
	th := test.SetupCommand(t)
	ctx := context.Background()

	// Create test DAGs
	dag1 := th.DAG(t, `name: filter-test-1
steps:
  - name: step1
    command: "echo test1"
`)

	dag2 := th.DAG(t, `name: filter-test-2
steps:
  - name: step2
    command: "echo test2"
`)

	// Execute both DAGs
	th.RunCommand(t, cmd.Start(), test.CmdTest{Args: []string{"start", dag1.Location}})
	th.RunCommand(t, cmd.Start(), test.CmdTest{Args: []string{"start", dag2.Location}})

	// Wait for both to complete
	require.Eventually(t, func() bool {
		s1, err1 := th.DAGRunMgr.GetLatestStatus(ctx, dag1.DAG)
		s2, err2 := th.DAGRunMgr.GetLatestStatus(ctx, dag2.DAG)
		return err1 == nil && err2 == nil && s1.Status == core.Succeeded && s2.Status == core.Succeeded
	}, 5*time.Second, 100*time.Millisecond)

	// Filter by specific DAG name
	th.RunCommand(t, cmd.History(), test.CmdTest{
		Name:        "FilterByName",
		Args:        []string{"history", "filter-test-1"},
		ExpectedOut: []string{"filter-test-1"},
	})

	// Verify the other DAG is not in output
	output := th.LoggingOutput.String()
	assert.NotContains(t, output, "filter-test-2")
}

func TestHistoryCommand_FilterByStatus(t *testing.T) {
	th := test.SetupCommand(t)
	ctx := context.Background()

	// Create successful and failing DAGs
	dagSuccess := th.DAG(t, `name: status-test-success
steps:
  - name: success-step
    command: "true"
`)

	dagFail := th.DAG(t, `name: status-test-fail
steps:
  - name: fail-step
    command: "false"
`)

	// Execute both
	th.RunCommand(t, cmd.Start(), test.CmdTest{Args: []string{"start", dagSuccess.Location}})
	_ = th.RunCommandWithError(t, cmd.Start(), test.CmdTest{Args: []string{"start", dagFail.Location}})

	// Wait for completion
	require.Eventually(t, func() bool {
		s1, err1 := th.DAGRunMgr.GetLatestStatus(ctx, dagSuccess.DAG)
		s2, err2 := th.DAGRunMgr.GetLatestStatus(ctx, dagFail.DAG)
		return err1 == nil && err2 == nil && s1.Status == core.Succeeded && s2.Status == core.Failed
	}, 5*time.Second, 100*time.Millisecond)

	// Filter by succeeded status
	th.RunCommand(t, cmd.History(), test.CmdTest{
		Name:        "FilterSucceeded",
		Args:        []string{"history", "--status=succeeded"},
		ExpectedOut: []string{"status-test-success", "Succeeded"},
	})

	output := th.LoggingOutput.String()
	assert.NotContains(t, output, "status-test-fail")
}

func TestHistoryCommand_JSONFormat(t *testing.T) {
	th := test.SetupCommand(t)
	ctx := context.Background()

	dag := th.DAG(t, `name: test-json-format
steps:
  - name: json-step
    command: "echo json"
`)

	th.RunCommand(t, cmd.Start(), test.CmdTest{Args: []string{"start", dag.Location}})

	require.Eventually(t, func() bool {
		status, err := th.DAGRunMgr.GetLatestStatus(ctx, dag.DAG)
		return err == nil && status.Status == core.Succeeded
	}, 5*time.Second, 100*time.Millisecond)

	// Test JSON output
	th.RunCommand(t, cmd.History(), test.CmdTest{
		Name: "JSONFormat",
		Args: []string{"history", "--format=json"},
		ExpectedOut: []string{
			`"name"`,
			`"dagRunId"`,
			`"status"`,
		},
	})
}

func TestHistoryCommand_RunIDDisplay(t *testing.T) {
	th := test.SetupCommand(t)
	ctx := context.Background()

	dag := th.DAG(t, `name: test-runid-full
steps:
  - name: simple-step
    command: "echo test"
`)

	// Execute with a long custom run ID
	customRunID := "custom-run-id-1234567890-abcdefghijklmnopqrstuvwxyz"
	th.RunCommand(t, cmd.Start(), test.CmdTest{
		Args: []string{"start", "--run-id=" + customRunID, dag.Location},
	})

	require.Eventually(t, func() bool {
		status, err := th.DAGRunMgr.GetCurrentStatus(ctx, dag.DAG, customRunID)
		return err == nil && status != nil && status.Status == core.Succeeded
	}, 5*time.Second, 100*time.Millisecond)

	// Verify full run ID is displayed
	th.RunCommand(t, cmd.History(), test.CmdTest{
		Name:        "FullRunID",
		Args:        []string{"history", "test-runid-full"},
		ExpectedOut: []string{customRunID},
	})

	output := th.LoggingOutput.String()
	assert.Contains(t, output, customRunID, "Full run ID must be displayed without truncation")
}

func TestHistoryCommand_DateFiltering(t *testing.T) {
	th := test.SetupCommand(t)
	ctx := context.Background()

	dag := th.DAG(t, `name: test-date-filter
steps:
  - name: simple-step
    command: "echo test"
`)

	th.RunCommand(t, cmd.Start(), test.CmdTest{Args: []string{"start", dag.Location}})

	require.Eventually(t, func() bool {
		status, err := th.DAGRunMgr.GetLatestStatus(ctx, dag.DAG)
		return err == nil && status.Status == core.Succeeded
	}, 5*time.Second, 100*time.Millisecond)

	// Test relative date filtering
	th.RunCommand(t, cmd.History(), test.CmdTest{
		Name:        "RelativeDate",
		Args:        []string{"history", "--last=1h"},
		ExpectedOut: []string{"test-date-filter"},
	})

	// Test absolute date filtering
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	th.RunCommand(t, cmd.History(), test.CmdTest{
		Name:        "AbsoluteDate",
		Args:        []string{"history", fmt.Sprintf("--from=%s", yesterday)},
		ExpectedOut: []string{"test-date-filter"},
	})
}

func TestHistoryCommand_Errors(t *testing.T) {
	th := test.SetupCommand(t)

	tests := []struct {
		name        string
		args        []string
		expectedErr string
	}{
		{
			name:        "InvalidStatus",
			args:        []string{"history", "--status=invalid"},
			expectedErr: "invalid status",
		},
		{
			name:        "InvalidDateFormat",
			args:        []string{"history", "--from=invalid-date"},
			expectedErr: "invalid --from date",
		},
		{
			name:        "InvalidLastDuration",
			args:        []string{"history", "--last=invalid"},
			expectedErr: "invalid --last value",
		},
		{
			name:        "ConflictingFlags",
			args:        []string{"history", "--last=7d", "--from=2026-01-01"},
			expectedErr: "cannot use --last with --from",
		},
		{
			name:        "InvalidLimit",
			args:        []string{"history", "--limit=invalid"},
			expectedErr: "invalid --limit value",
		},
		{
			name:        "InvalidFormat",
			args:        []string{"history", "--format=pdf"},
			expectedErr: "invalid format",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := th.RunCommandWithError(t, cmd.History(), test.CmdTest{
				Name: tc.name,
				Args: tc.args,
			})

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

func TestHistoryCommand_EmptyResults(t *testing.T) {
	th := test.SetupCommand(t)

	// Query for non-existent DAG
	th.RunCommand(t, cmd.History(), test.CmdTest{
		Name:        "NonExistent",
		Args:        []string{"history", "non-existent-dag-xyz"},
		ExpectedOut: []string{"No DAG runs found"},
	})
}

func TestHistoryCommand_Tags(t *testing.T) {
	th := test.SetupCommand(t)
	ctx := context.Background()

	// Create DAGs with different tags
	dag1 := th.DAG(t, `name: tagged-dag-1
tags:
  - prod
  - critical
steps:
  - name: step1
    command: "echo test"
`)

	dag2 := th.DAG(t, `name: tagged-dag-2
tags:
  - dev
steps:
  - name: step2
    command: "echo test"
`)

	th.RunCommand(t, cmd.Start(), test.CmdTest{Args: []string{"start", dag1.Location}})
	th.RunCommand(t, cmd.Start(), test.CmdTest{Args: []string{"start", dag2.Location}})

	require.Eventually(t, func() bool {
		s1, err1 := th.DAGRunMgr.GetLatestStatus(ctx, dag1.DAG)
		s2, err2 := th.DAGRunMgr.GetLatestStatus(ctx, dag2.DAG)
		return err1 == nil && err2 == nil && s1.Status == core.Succeeded && s2.Status == core.Succeeded
	}, 5*time.Second, 100*time.Millisecond)

	// Filter by tag
	th.RunCommand(t, cmd.History(), test.CmdTest{
		Name:        "FilterByTag",
		Args:        []string{"history", "--tags=prod"},
		ExpectedOut: []string{"tagged-dag-1"},
	})

	output := th.LoggingOutput.String()
	assert.NotContains(t, output, "tagged-dag-2")
}

func TestHistoryCommand_Limit(t *testing.T) {
	th := test.SetupCommand(t)
	ctx := context.Background()

	dag := th.DAG(t, `name: test-limit
steps:
  - name: step
    command: "echo test"
`)

	// Create multiple runs
	for i := 0; i < 3; i++ {
		th.RunCommand(t, cmd.Start(), test.CmdTest{Args: []string{"start", dag.Location}})
		time.Sleep(100 * time.Millisecond)
	}

	require.Eventually(t, func() bool {
		statuses, _ := th.DAGRunStore.ListStatuses(ctx)
		count := 0
		for _, s := range statuses {
			if s.Name == "test-limit" && s.Status == core.Succeeded {
				count++
			}
		}
		return count >= 3
	}, 10*time.Second, 200*time.Millisecond)

	// Test limit
	th.RunCommand(t, cmd.History(), test.CmdTest{
		Name:        "LimitResults",
		Args:        []string{"history", "test-limit", "--limit=2"},
		ExpectedOut: []string{"test-limit"},
	})

	// Count occurrences in output
	output := th.LoggingOutput.String()
	lines := strings.Split(output, "\n")
	count := 0
	for _, line := range lines {
		if strings.Contains(line, "test-limit") && !strings.Contains(line, "DAG NAME") {
			count++
		}
	}
	assert.LessOrEqual(t, count, 2, "Should respect limit of 2")
}
