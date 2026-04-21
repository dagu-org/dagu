// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	openapiv1 "github.com/dagucloud/dagu/api/v1"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func labelsFromPatchedSpec(t *testing.T, data []byte) []any {
	t.Helper()

	var firstDoc yaml.MapSlice
	require.NoError(t, yaml.Unmarshal(data, &firstDoc))

	raw, ok := getInlineEnqueueMapValue(firstDoc, "labels")
	require.True(t, ok)

	labels, ok := raw.([]any)
	require.True(t, ok)
	return labels
}

func requireNoDeprecatedTagsKey(t *testing.T, data []byte) {
	t.Helper()

	var firstDoc yaml.MapSlice
	require.NoError(t, yaml.Unmarshal(data, &firstDoc))

	_, ok := getInlineEnqueueMapValue(firstDoc, "tags")
	require.False(t, ok)
}

func TestApplyInlineEnqueueLabels_ArrayLabels(t *testing.T) {
	t.Parallel()

	data := []byte(`name: test
labels:
  - env=prod
steps:
  - name: s1
    command: echo hi
`)

	patched, err := applyInlineEnqueueLabels(data, "team=backend")
	require.NoError(t, err)

	labels := labelsFromPatchedSpec(t, patched)
	assert.Contains(t, labels, "env=prod")
	assert.Contains(t, labels, "team=backend")
	requireNoDeprecatedTagsKey(t, patched)
}

func TestApplyInlineEnqueueLabels_CommaSeparatedStringLabels(t *testing.T) {
	t.Parallel()

	data := []byte(`name: test
labels: "daily, weekly"
steps:
  - name: s1
    command: echo hi
`)

	patched, err := applyInlineEnqueueLabels(data, "team=backend")
	require.NoError(t, err)

	labels := labelsFromPatchedSpec(t, patched)
	assert.Contains(t, labels, "daily")
	assert.Contains(t, labels, "weekly")
	assert.Contains(t, labels, "team=backend")
	requireNoDeprecatedTagsKey(t, patched)
}

func TestApplyInlineEnqueueLabels_SpaceSeparatedKeyValueLabels(t *testing.T) {
	t.Parallel()

	data := []byte(`name: test
labels: "env=prod team=platform"
steps:
  - name: s1
    command: echo hi
`)

	patched, err := applyInlineEnqueueLabels(data, "team=backend")
	require.NoError(t, err)

	labels := labelsFromPatchedSpec(t, patched)
	assert.Contains(t, labels, "env=prod")
	assert.Contains(t, labels, "team=platform")
	assert.Contains(t, labels, "team=backend")
	requireNoDeprecatedTagsKey(t, patched)
}

func TestApplyInlineEnqueueLabels_MapLabels(t *testing.T) {
	t.Parallel()

	data := []byte(`name: test
labels:
  env: prod
  team: platform
steps:
  - name: s1
    command: echo hi
`)

	patched, err := applyInlineEnqueueLabels(data, "priority=high")
	require.NoError(t, err)

	labels := labelsFromPatchedSpec(t, patched)
	assert.Contains(t, labels, "env=prod")
	assert.Contains(t, labels, "team=platform")
	assert.Contains(t, labels, "priority=high")
	requireNoDeprecatedTagsKey(t, patched)
}

func TestApplyInlineEnqueueLabels_DeprecatedTagsCanonicalizeToLabels(t *testing.T) {
	t.Parallel()

	data := []byte(`name: test
tags:
  - env=prod
steps:
  - name: s1
    command: echo hi
`)

	patched, err := applyInlineEnqueueLabels(data, "team=backend")
	require.NoError(t, err)

	labels := labelsFromPatchedSpec(t, patched)
	assert.Contains(t, labels, "env=prod")
	assert.Contains(t, labels, "team=backend")
	requireNoDeprecatedTagsKey(t, patched)
}

func TestApplyInlineEnqueueLabels_PreservesLaterDocuments(t *testing.T) {
	t.Parallel()

	data := []byte(`name: main
steps:
  - name: s1
    command: echo hi
---
name: child
steps:
  - name: s2
    command: echo bye
`)

	patched, err := applyInlineEnqueueLabels(data, "env=prod")
	require.NoError(t, err)

	content := string(patched)
	assert.Contains(t, content, "labels:")
	assert.Contains(t, content, "env=prod")
	assert.Contains(t, content, "---")
	assert.True(t, strings.Contains(content, "name: child") || strings.Contains(content, "name: \"child\""))
	assert.Contains(t, content, "echo bye")
	requireNoDeprecatedTagsKey(t, patched)
}

func TestApplyInlineEnqueueLabels_InvalidYAML(t *testing.T) {
	t.Parallel()

	_, err := applyInlineEnqueueLabels([]byte("{{invalid yaml"), "env=prod")
	require.Error(t, err)
}

func TestDAGRunListOptionsFromQueryStringParsesMultipleStatuses(t *testing.T) {
	t.Parallel()

	api := &API{}
	opts, err := api.dagRunListOptionsFromQueryString(
		context.Background(),
		"status=5&status=1,6&limit=20",
	)
	require.NoError(t, err)

	var applied exec.ListDAGRunStatusesOptions
	for _, opt := range opts.query {
		opt(&applied)
	}

	require.Equal(t, []core.Status{
		core.Status(openapiv1.StatusQueued),
		core.Status(openapiv1.StatusRunning),
		core.Status(openapiv1.StatusPartialSuccess),
	}, applied.Statuses)
	require.Equal(t, 20, applied.Limit)
}

func TestDAGRunListOptionsFromQueryStringRejectsInvalidStatuses(t *testing.T) {
	t.Parallel()

	api := &API{}
	_, err := api.dagRunListOptionsFromQueryString(
		context.Background(),
		"status=1&status=running",
	)
	require.Error(t, err)

	apiErr, ok := err.(*Error)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, apiErr.HTTPStatus)
	require.Equal(t, openapiv1.ErrorCodeBadRequest, apiErr.Code)
	require.Contains(t, apiErr.Message, "invalid status parameter")
}

var _ exec.DAGRunStore = (*blockingDAGRunStore)(nil)

type blockingDAGRunStore struct{}

func (blockingDAGRunStore) CreateAttempt(context.Context, *core.DAG, time.Time, string, exec.NewDAGRunAttemptOptions) (exec.DAGRunAttempt, error) {
	panic("not implemented")
}

func (blockingDAGRunStore) RecentAttempts(context.Context, string, int) []exec.DAGRunAttempt {
	panic("not implemented")
}

func (blockingDAGRunStore) LatestAttempt(context.Context, string) (exec.DAGRunAttempt, error) {
	panic("not implemented")
}

func (blockingDAGRunStore) ListStatuses(ctx context.Context, _ ...exec.ListDAGRunStatusesOption) ([]*exec.DAGRunStatus, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (blockingDAGRunStore) ListStatusesPage(ctx context.Context, _ ...exec.ListDAGRunStatusesOption) (exec.DAGRunStatusPage, error) {
	<-ctx.Done()
	return exec.DAGRunStatusPage{}, ctx.Err()
}

func (blockingDAGRunStore) CompareAndSwapLatestAttemptStatus(context.Context, exec.DAGRunRef, string, core.Status, func(*exec.DAGRunStatus) error) (*exec.DAGRunStatus, bool, error) {
	panic("not implemented")
}

func (blockingDAGRunStore) FindAttempt(context.Context, exec.DAGRunRef) (exec.DAGRunAttempt, error) {
	panic("not implemented")
}

func (blockingDAGRunStore) FindSubAttempt(context.Context, exec.DAGRunRef, string) (exec.DAGRunAttempt, error) {
	panic("not implemented")
}

func (blockingDAGRunStore) CreateSubAttempt(context.Context, exec.DAGRunRef, string) (exec.DAGRunAttempt, error) {
	panic("not implemented")
}

func (blockingDAGRunStore) RemoveOldDAGRuns(context.Context, string, int, ...exec.RemoveOldDAGRunsOption) ([]string, error) {
	panic("not implemented")
}

func (blockingDAGRunStore) RenameDAGRuns(context.Context, string, string) error {
	panic("not implemented")
}

func (blockingDAGRunStore) RemoveDAGRun(context.Context, exec.DAGRunRef, ...exec.RemoveDAGRunOption) error {
	panic("not implemented")
}

func TestAPIListDAGRunsReturnsGatewayTimeoutWhenReadDeadlineExpires(t *testing.T) {
	t.Parallel()

	api := &API{
		dagRunStore: blockingDAGRunStore{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	resp, err := api.ListDAGRuns(ctx, openapiv1.ListDAGRunsRequestObject{})
	require.NoError(t, err)

	timeoutResp, ok := resp.(openapiv1.ListDAGRunsdefaultJSONResponse)
	require.True(t, ok)
	require.Equal(t, http.StatusGatewayTimeout, timeoutResp.StatusCode)
	require.Equal(t, openapiv1.ErrorCodeTimeout, timeoutResp.Body.Code)
	require.Equal(t, "dag-run list request timed out", timeoutResp.Body.Message)
}

func TestDAGRunListOptionsFromQueryStringIncludesWorkspaceFilter(t *testing.T) {
	t.Parallel()

	api := &API{}

	t.Run("workspace scope", func(t *testing.T) {
		t.Parallel()

		opts, err := api.dagRunListOptionsFromQueryString(
			context.Background(),
			"workspaceScope=workspace&workspace=ops",
		)
		require.NoError(t, err)

		var listOpts exec.ListDAGRunStatusesOptions
		for _, opt := range opts.query {
			opt(&listOpts)
		}

		require.NotNil(t, listOpts.WorkspaceFilter)
		assert.True(t, listOpts.WorkspaceFilter.Enabled)
		assert.Equal(t, []string{"ops"}, listOpts.WorkspaceFilter.Workspaces)
		assert.False(t, listOpts.WorkspaceFilter.IncludeUnlabelled)
	})

	t.Run("default scope", func(t *testing.T) {
		t.Parallel()

		opts, err := api.dagRunListOptionsFromQueryString(
			context.Background(),
			"workspaceScope=default",
		)
		require.NoError(t, err)

		var listOpts exec.ListDAGRunStatusesOptions
		for _, opt := range opts.query {
			opt(&listOpts)
		}

		require.NotNil(t, listOpts.WorkspaceFilter)
		assert.True(t, listOpts.WorkspaceFilter.Enabled)
		assert.Empty(t, listOpts.WorkspaceFilter.Workspaces)
		assert.True(t, listOpts.WorkspaceFilter.IncludeUnlabelled)
	})

	t.Run("all scope without auth keeps aggregate unfiltered", func(t *testing.T) {
		t.Parallel()

		opts, err := api.dagRunListOptionsFromQueryString(
			context.Background(),
			"workspaceScope=all",
		)
		require.NoError(t, err)

		var listOpts exec.ListDAGRunStatusesOptions
		for _, opt := range opts.query {
			opt(&listOpts)
		}

		assert.Nil(t, listOpts.WorkspaceFilter)
	})
}

type blockingLatestAttemptStore struct {
	blockingDAGRunStore
}

func (blockingLatestAttemptStore) LatestAttempt(ctx context.Context, _ string) (exec.DAGRunAttempt, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestWithDAGRunReadTimeoutReturnsDeadlineExceededOnLateSuccess(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	_, err := withDAGRunReadTimeout(ctx, dagRunReadRequestInfo{
		endpoint: "/dag-runs/{name}/{dagRunId}",
	}, func(readCtx context.Context) (string, error) {
		<-readCtx.Done()
		return "late-success", nil
	})

	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestGetDAGRunDetailsReturnsClientClosedRequestWhenReadCanceled(t *testing.T) {
	t.Parallel()

	api := &API{
		dagRunStore: blockingLatestAttemptStore{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resp, err := api.GetDAGRunDetails(ctx, openapiv1.GetDAGRunDetailsRequestObject{
		Name:     "test",
		DagRunId: "latest",
	})
	require.NoError(t, err)

	canceledResp, ok := resp.(*openapiv1.GetDAGRunDetailsdefaultJSONResponse)
	require.True(t, ok)
	require.Equal(t, statusClientClosedRequest, canceledResp.StatusCode)
	require.Equal(t, openapiv1.ErrorCodeInternalError, canceledResp.Body.Code)
	require.Equal(t, "dag-run details request canceled", canceledResp.Body.Message)
}

func TestRebuildDAGRunSnapshotFromYAMLRestoresHarnessConfig(t *testing.T) {
	t.Parallel()

	dag := &core.DAG{
		Name: "snapshot-harness",
		YamlData: []byte(`
harnesses:
  gemini:
    binary: gemini
    prefix_args: ["run"]
    prompt_mode: flag
    prompt_flag: --prompt
harness:
  provider: gemini
  model: gemini-2.5-pro
  fallback:
    - provider: claude
      model: sonnet
steps:
  - command: "Review the repository"
`),
	}

	restored, err := rebuildDAGRunSnapshotFromYAML(context.Background(), dag)
	require.NoError(t, err)
	require.Same(t, dag, restored)

	require.NotNil(t, restored.Harness)
	assert.Equal(t, "gemini", restored.Harness.Config["provider"])
	assert.Equal(t, "gemini-2.5-pro", restored.Harness.Config["model"])
	require.Len(t, restored.Harness.Fallback, 1)
	assert.Equal(t, "claude", restored.Harness.Fallback[0]["provider"])

	require.NotNil(t, restored.Harnesses)
	require.Contains(t, restored.Harnesses, "gemini")
	require.NotNil(t, restored.Harnesses["gemini"])
	assert.Equal(t, "gemini", restored.Harnesses["gemini"].Binary)
	assert.Equal(t, core.HarnessPromptModeFlag, restored.Harnesses["gemini"].PromptMode)
	assert.Equal(t, "--prompt", restored.Harnesses["gemini"].PromptFlag)
}
