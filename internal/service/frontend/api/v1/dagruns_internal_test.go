// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"strings"
	"testing"

	apiv1 "github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type latestAttemptStatusSwapperStub struct {
	compareAndSwap func(
		ctx context.Context,
		dagRun exec.DAGRunRef,
		expectedAttemptID string,
		expectedStatus core.Status,
		mutate func(*exec.DAGRunStatus) error,
	) (*exec.DAGRunStatus, bool, error)
}

func (s latestAttemptStatusSwapperStub) CompareAndSwapLatestAttemptStatus(
	ctx context.Context,
	dagRun exec.DAGRunRef,
	expectedAttemptID string,
	expectedStatus core.Status,
	mutate func(*exec.DAGRunStatus) error,
) (*exec.DAGRunStatus, bool, error) {
	return s.compareAndSwap(ctx, dagRun, expectedAttemptID, expectedStatus, mutate)
}

func tagsFromPatchedSpec(t *testing.T, data []byte) []any {
	t.Helper()

	var firstDoc yaml.MapSlice
	require.NoError(t, yaml.Unmarshal(data, &firstDoc))

	raw, ok := getInlineEnqueueMapValue(firstDoc, "tags")
	require.True(t, ok)

	tags, ok := raw.([]any)
	require.True(t, ok)
	return tags
}

func TestApplyInlineEnqueueTags_ArrayTags(t *testing.T) {
	t.Parallel()

	data := []byte(`name: test
tags:
  - env=prod
steps:
  - name: s1
    command: echo hi
`)

	patched, err := applyInlineEnqueueTags(data, "team=backend")
	require.NoError(t, err)

	tags := tagsFromPatchedSpec(t, patched)
	assert.Contains(t, tags, "env=prod")
	assert.Contains(t, tags, "team=backend")
}

func TestApplyInlineEnqueueTags_CommaSeparatedStringTags(t *testing.T) {
	t.Parallel()

	data := []byte(`name: test
tags: "daily, weekly"
steps:
  - name: s1
    command: echo hi
`)

	patched, err := applyInlineEnqueueTags(data, "team=backend")
	require.NoError(t, err)

	tags := tagsFromPatchedSpec(t, patched)
	assert.Contains(t, tags, "daily")
	assert.Contains(t, tags, "weekly")
	assert.Contains(t, tags, "team=backend")
}

func TestApplyInlineEnqueueTags_SpaceSeparatedKeyValueTags(t *testing.T) {
	t.Parallel()

	data := []byte(`name: test
tags: "env=prod team=platform"
steps:
  - name: s1
    command: echo hi
`)

	patched, err := applyInlineEnqueueTags(data, "team=backend")
	require.NoError(t, err)

	tags := tagsFromPatchedSpec(t, patched)
	assert.Contains(t, tags, "env=prod")
	assert.Contains(t, tags, "team=platform")
	assert.Contains(t, tags, "team=backend")
}

func TestApplyInlineEnqueueTags_MapTags(t *testing.T) {
	t.Parallel()

	data := []byte(`name: test
tags:
  env: prod
  team: platform
steps:
  - name: s1
    command: echo hi
`)

	patched, err := applyInlineEnqueueTags(data, "priority=high")
	require.NoError(t, err)

	tags := tagsFromPatchedSpec(t, patched)
	assert.Contains(t, tags, "env=prod")
	assert.Contains(t, tags, "team=platform")
	assert.Contains(t, tags, "priority=high")
}

func TestApplyInlineEnqueueTags_PreservesLaterDocuments(t *testing.T) {
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

	patched, err := applyInlineEnqueueTags(data, "env=prod")
	require.NoError(t, err)

	content := string(patched)
	assert.Contains(t, content, "tags:")
	assert.Contains(t, content, "env=prod")
	assert.Contains(t, content, "---")
	assert.True(t, strings.Contains(content, "name: child") || strings.Contains(content, "name: \"child\""))
	assert.Contains(t, content, "echo bye")
}

func TestApplyInlineEnqueueTags_InvalidYAML(t *testing.T) {
	t.Parallel()

	_, err := applyInlineEnqueueTags([]byte("{{invalid yaml"), "env=prod")
	require.Error(t, err)
}

func TestCancelFailedAutoRetryPendingDAGRunMutatesToAborted(t *testing.T) {
	t.Parallel()

	ref := exec.NewDAGRunRef("test", "run-1")
	err := cancelFailedAutoRetryPendingDAGRun(
		context.Background(),
		latestAttemptStatusSwapperStub{
			compareAndSwap: func(
				_ context.Context,
				dagRun exec.DAGRunRef,
				expectedAttemptID string,
				expectedStatus core.Status,
				mutate func(*exec.DAGRunStatus) error,
			) (*exec.DAGRunStatus, bool, error) {
				assert.Equal(t, ref, dagRun)
				assert.Equal(t, "attempt-1", expectedAttemptID)
				assert.Equal(t, core.Failed, expectedStatus)

				status := &exec.DAGRunStatus{Status: core.Failed}
				require.NoError(t, mutate(status))
				assert.Equal(t, core.Aborted, status.Status)
				return status, true, nil
			},
		},
		ref,
		"attempt-1",
	)
	require.NoError(t, err)
}

func TestCancelFailedAutoRetryPendingDAGRunReturnsRefreshableErrorOnStateChange(t *testing.T) {
	t.Parallel()

	err := cancelFailedAutoRetryPendingDAGRun(
		context.Background(),
		latestAttemptStatusSwapperStub{
			compareAndSwap: func(
				_ context.Context,
				_ exec.DAGRunRef,
				_ string,
				_ core.Status,
				_ func(*exec.DAGRunStatus) error,
			) (*exec.DAGRunStatus, bool, error) {
				return &exec.DAGRunStatus{Status: core.Queued}, false, nil
			},
		},
		exec.NewDAGRunRef("test", "run-1"),
		"attempt-1",
	)
	require.Error(t, err)

	apiErr, ok := err.(*Error)
	require.True(t, ok)
	assert.Equal(t, apiv1.ErrorCodeBadRequest, apiErr.Code)
	assert.Contains(t, apiErr.Message, "current status is queued")
	assert.Contains(t, apiErr.Message, "Refresh and try again")
}
