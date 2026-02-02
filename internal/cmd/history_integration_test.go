package cmd_test

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

func TestHistoryCommand(t *testing.T) {
	// Setup test environment
	th := test.SetupCommand(t)
	ctx := context.Background()

	// Create multiple test DAGs with different configurations
	dagSucceeded := th.DAG(t, `name: test-succeeded
tags:
  - prod
  - critical
steps:
  - name: success-step
    command: "echo success"
`)

	dagFailed := th.DAG(t, `name: test-failed
tags:
  - dev
steps:
  - name: fail-step
    command: "false"
`)

	dagWithParams := th.DAG(t, `name: test-params
params: "key1=value1 key2=value2"
tags:
  - prod
steps:
  - name: params-step
    command: "echo $key1 $key2"
`)

	// Execute DAGs to create history
	t.Run("Setup: Create DAG run history", func(t *testing.T) {
		// Run succeeded DAG
		th.RunCommand(t, cmd.Start(), test.CmdTest{
			Args: []string{"start", dagSucceeded.Location},
		})

		// Wait for completion
		require.Eventually(t, func() bool {
			status, err := th.DAGRunMgr.GetLatestStatus(ctx, dagSucceeded.DAG)
			return err == nil && status.Status == core.Succeeded
		}, 5*time.Second, 100*time.Millisecond)

		// Run failed DAG
		th.RunCommandWithError(t, cmd.Start(), test.CmdTest{
			Args: []string{"start", dagFailed.Location},
		})

		// Wait for completion
		require.Eventually(t, func() bool {
			status, err := th.DAGRunMgr.GetLatestStatus(ctx, dagFailed.DAG)
			return err == nil && status.Status == core.Failed
		}, 5*time.Second, 100*time.Millisecond)

		// Run DAG with params (succeeded)
		th.RunCommand(t, cmd.Start(), test.CmdTest{
			Args: []string{"start", dagWithParams.Location},
		})

		// Wait for completion
		require.Eventually(t, func() bool {
			status, err := th.DAGRunMgr.GetLatestStatus(ctx, dagWithParams.DAG)
			return err == nil && status.Status == core.Succeeded
		}, 5*time.Second, 100*time.Millisecond)
	})

	// Test cases for history command
	tests := []struct {
		name        string
		args        []string
		expectedOut []string
		notExpected []string
	}{
		{
			name:        "ShowAllHistory",
			args:        []string{"history"},
			expectedOut: []string{"test-succeeded", "test-failed", "test-params", "DAG NAME", "RUN ID", "STATUS"},
		},
		{
			name:        "FilterByDAGName",
			args:        []string{"history", "test-succeeded"},
			expectedOut: []string{"test-succeeded", "Succeeded"},
			notExpected: []string{"test-failed", "test-params"},
		},
		{
			name:        "FilterByStatusSucceeded",
			args:        []string{"history", "--status=succeeded"},
			expectedOut: []string{"test-succeeded", "test-params", "Succeeded"},
			notExpected: []string{"Failed"},
		},
		{
			name:        "FilterByStatusFailed",
			args:        []string{"history", "--status=failed"},
			expectedOut: []string{"test-failed", "Failed"},
			notExpected: []string{"test-succeeded", "test-params"},
		},
		{
			name:        "FilterByTagsSingle",
			args:        []string{"history", "--tags=prod"},
			expectedOut: []string{"test-succeeded", "test-params"},
			notExpected: []string{"test-failed"},
		},
		{
			name:        "FilterByTagsMultiple",
			args:        []string{"history", "--tags=prod,critical"},
			expectedOut: []string{"test-succeeded"},
			notExpected: []string{"test-failed", "test-params"},
		},
		{
			name:        "LimitResults",
			args:        []string{"history", "--limit=1"},
			expectedOut: []string{"DAG NAME", "RUN ID"},
		},
		{
			name:        "JSONFormat",
			args:        []string{"history", "--format=json"},
			expectedOut: []string{`"name"`, `"dagRunId"`, `"status"`},
		},
		{
			name:        "CombinedFilters",
			args:        []string{"history", "--status=succeeded", "--tags=prod", "--limit=5"},
			expectedOut: []string{"test-succeeded", "test-params"},
			notExpected: []string{"test-failed"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Run history command
			th.RunCommand(t, cmd.History(), test.CmdTest{
				Name:        tc.name,
				Args:        tc.args,
				ExpectedOut: tc.expectedOut,
			})

			// Check that unexpected strings are not in output
			if len(tc.notExpected) > 0 {
				output := th.LoggingOutput.String()
				for _, notExpected := range tc.notExpected {
					assert.NotContains(t, output, notExpected, "Output should not contain: %s", notExpected)
				}
			}
		})
	}
}

func TestHistoryCommand_RunIDDisplay(t *testing.T) {
	// This test specifically verifies that run IDs are displayed in full
	th := test.SetupCommand(t)

	dag := th.DAG(t, `name: test-runid-display
steps:
  - name: simple-step
    command: "echo test"
`)

	// Execute DAG with a specific run ID
	customRunID := "custom-run-id-1234567890-abcdefghijklmnopqrstuvwxyz"
	th.RunCommand(t, cmd.Start(), test.CmdTest{
		Args: []string{"start", "--run-id=" + customRunID, dag.Location},
	})

	// Wait for completion
	ctx := context.Background()
	require.Eventually(t, func() bool {
		status, err := th.DAGRunMgr.GetCurrentStatus(ctx, dag.DAG, customRunID)
		return err == nil && status != nil && status.Status == core.Succeeded
	}, 5*time.Second, 100*time.Millisecond)

	// Run history command and verify full run ID is displayed
	th.RunCommand(t, cmd.History(), test.CmdTest{
		Name: "VerifyFullRunID",
		Args: []string{"history", "test-runid-display"},
		ExpectedOut: []string{
			customRunID, // Full run ID should be present
		},
	})

	output := th.LoggingOutput.String()

	// Verify the run ID is NOT truncated
	assert.Contains(t, output, customRunID, "Full run ID must be displayed")

	// Verify we can find the run by partial ID
	th.RunCommand(t, cmd.History(), test.CmdTest{
		Name: "SearchByPartialRunID",
		Args: []string{"history", "--run-id=custom-run-id"},
		ExpectedOut: []string{
			customRunID,
			"test-runid-display",
		},
	})
}

func TestHistoryCommand_DateFiltering(t *testing.T) {
	th := test.SetupCommand(t)
	ctx := context.Background()

	dag := th.DAG(t, `name: test-date-filter
steps:
  - name: simple-step
    command: "echo test"
`)

	// Create a run
	th.RunCommand(t, cmd.Start(), test.CmdTest{
		Args: []string{"start", dag.Location},
	})

	// Wait for completion
	require.Eventually(t, func() bool {
		status, err := th.DAGRunMgr.GetLatestStatus(ctx, dag.DAG)
		return err == nil && status.Status == core.Succeeded
	}, 5*time.Second, 100*time.Millisecond)

	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1)
	tomorrow := now.AddDate(0, 0, 1)

	tests := []struct {
		name        string
		args        []string
		shouldFind  bool
		description string
	}{
		{
			name:        "RelativeLast1Hour",
			args:        []string{"history", "--last=1h"},
			shouldFind:  true,
			description: "Should find run from last hour",
		},
		{
			name:        "RelativeLast1Day",
			args:        []string{"history", "--last=1d"},
			shouldFind:  true,
			description: "Should find run from last day",
		},
		{
			name:        "AbsoluteFromYesterday",
			args:        []string{"history", fmt.Sprintf("--from=%s", yesterday.Format("2006-01-02"))},
			shouldFind:  true,
			description: "Should find run from yesterday onwards",
		},
		{
			name:        "AbsoluteToTomorrow",
			args:        []string{"history", fmt.Sprintf("--to=%s", tomorrow.Format("2006-01-02"))},
			shouldFind:  true,
			description: "Should find run up to tomorrow",
		},
		{
			name: "AbsoluteDateRange",
			args: []string{
				"history",
				fmt.Sprintf("--from=%s", yesterday.Format("2006-01-02")),
				fmt.Sprintf("--to=%s", tomorrow.Format("2006-01-02")),
			},
			shouldFind:  true,
			description: "Should find run in date range",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := th.RunCommandWithError(t, cmd.History(), test.CmdTest{
				Name: tc.name,
				Args: tc.args,
			})

			require.NoError(t, err)

			output := th.LoggingOutput.String()

			if tc.shouldFind {
				assert.Contains(t, output, "test-date-filter", tc.description)
			} else {
				if !strings.Contains(output, "No DAG runs found") {
					assert.NotContains(t, output, "test-date-filter", tc.description)
				}
			}
		})
	}
}

func TestHistoryCommand_ErrorCases(t *testing.T) {
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
			args:        []string{"history", "--from=2026-13-01"},
			expectedErr: "invalid --from date",
		},
		{
			name:        "InvalidLastDuration",
			args:        []string{"history", "--last=7x"},
			expectedErr: "invalid --last value",
		},
		{
			name:        "ConflictingDateFlags",
			args:        []string{"history", "--last=7d", "--from=2026-01-01"},
			expectedErr: "cannot use --last with --from",
		},
		{
			name:        "InvalidLimit",
			args:        []string{"history", "--limit=invalid"},
			expectedErr: "invalid --limit value",
		},
		{
			name:        "NegativeLimit",
			args:        []string{"history", "--limit=-5"},
			expectedErr: "invalid --limit value",
		},
		{
			name:        "InvalidFormat",
			args:        []string{"history", "--format=xml"},
			expectedErr: "invalid format",
		},
		{
			name:        "FromAfterTo",
			args:        []string{"history", "--from=2026-02-01", "--to=2026-01-01"},
			expectedErr: "must be before",
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

	// Query for a DAG that doesn't exist
	th.RunCommand(t, cmd.History(), test.CmdTest{
		Name:        "NonExistentDAG",
		Args:        []string{"history", "non-existent-dag-name-12345"},
		ExpectedOut: []string{"No DAG runs found"},
	})
}

func TestHistoryCommand_JSONFormat(t *testing.T) {
	th := test.SetupCommand(t)
	ctx := context.Background()

	dag := th.DAG(t, `name: test-json-output
tags:
  - test
  - integration
steps:
  - name: json-step
    command: "echo json test"
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

	// Run history command with JSON format
	th.RunCommand(t, cmd.History(), test.CmdTest{
		Name: "JSONOutput",
		Args: []string{"history", "test-json-output", "--format=json"},
		ExpectedOut: []string{
			`"name": "test-json-output"`,
			`"dagRunId"`,
			`"status"`,
			`"startedAt"`,
			`"finishedAt"`,
			`"duration"`,
			`"tags"`,
		},
	})

	output := th.LoggingOutput.String()

	// Verify JSON structure
	assert.Contains(t, output, `[`)
	assert.Contains(t, output, `]`)
	assert.Contains(t, output, `{`)
	assert.Contains(t, output, `}`)

	// Verify required fields are present
	assert.Contains(t, output, `"succeeded"`) // Status should be in JSON
}

func TestHistoryCommand_ParameterDisplay(t *testing.T) {
	th := test.SetupCommand(t)
	ctx := context.Background()

	dag := th.DAG(t, `name: test-params-display
params: "default1=val1 default2=val2"
steps:
  - name: param-step
    command: "echo $default1 $default2"
`)

	// Execute DAG with parameters
	th.RunCommand(t, cmd.Start(), test.CmdTest{
		Args: []string{"start", "--params=override1=new1 override2=new2", dag.Location},
	})

	// Wait for completion
	require.Eventually(t, func() bool {
		status, err := th.DAGRunMgr.GetLatestStatus(ctx, dag.DAG)
		return err == nil && status.Status == core.Succeeded
	}, 5*time.Second, 100*time.Millisecond)

	// Run history command
	th.RunCommand(t, cmd.History(), test.CmdTest{
		Name:        "ParamsDisplay",
		Args:        []string{"history", "test-params-display"},
		ExpectedOut: []string{"test-params-display", "Succeeded"},
	})

	// In table format, params might be truncated, but in JSON they should be full
	th.RunCommand(t, cmd.History(), test.CmdTest{
		Name: "ParamsDisplayJSON",
		Args: []string{"history", "test-params-display", "--format=json"},
		ExpectedOut: []string{
			`"params"`,
		},
	})
}
