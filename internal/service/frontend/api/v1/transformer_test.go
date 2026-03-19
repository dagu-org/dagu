// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"testing"

	openapi "github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToDAGRunSummaryIncludesScheduleTime(t *testing.T) {
	status := exec.DAGRunStatus{
		Name:           "test-dag",
		DAGRunID:       "run-1",
		AutoRetryCount: 2,
		AutoRetryLimit: 5,
		Status:         core.Queued,
		ScheduleTime:   "2026-03-13T00:00:00Z",
	}

	summary := toDAGRunSummary(status)
	require.NotNil(t, summary.ScheduleTime)
	assert.Equal(t, status.ScheduleTime, *summary.ScheduleTime)
	assert.Equal(t, status.AutoRetryCount, summary.AutoRetryCount)
	require.NotNil(t, summary.AutoRetryLimit)
	assert.Equal(t, status.AutoRetryLimit, *summary.AutoRetryLimit)
}

func TestToDAGRunDetailsIncludesScheduleTime(t *testing.T) {
	status := exec.DAGRunStatus{
		Name:           "test-dag",
		DAGRunID:       "run-1",
		AutoRetryCount: 3,
		AutoRetryLimit: 5,
		Status:         core.Queued,
		QueuedAt:       "2026-03-13T00:01:00Z",
		ScheduleTime:   "2026-03-13T00:00:00Z",
	}

	details := ToDAGRunDetails(status)
	require.NotNil(t, details.ScheduleTime)
	assert.Equal(t, status.ScheduleTime, *details.ScheduleTime)
	require.NotNil(t, details.QueuedAt)
	assert.Equal(t, status.QueuedAt, *details.QueuedAt)
	assert.Equal(t, status.AutoRetryCount, details.AutoRetryCount)
	require.NotNil(t, details.AutoRetryLimit)
	assert.Equal(t, status.AutoRetryLimit, *details.AutoRetryLimit)
}

func TestToDAGRunSummaryOmitsAutoRetryLimitWhenUnconfigured(t *testing.T) {
	status := exec.DAGRunStatus{
		Name:           "test-dag",
		DAGRunID:       "run-1",
		AutoRetryCount: 0,
		AutoRetryLimit: 0,
		Status:         core.Failed,
	}

	summary := toDAGRunSummary(status)
	assert.Nil(t, summary.AutoRetryLimit)
}

func TestToDAGRunDetailsOmitsAutoRetryLimitWhenUnconfigured(t *testing.T) {
	status := exec.DAGRunStatus{
		Name:           "test-dag",
		DAGRunID:       "run-1",
		AutoRetryCount: 0,
		AutoRetryLimit: 0,
		Status:         core.Failed,
	}

	details := ToDAGRunDetails(status)
	assert.Nil(t, details.AutoRetryLimit)
}

func TestToDAGDetailsIncludesParamDefDescriptions(t *testing.T) {
	details := toDAGDetails(&core.DAG{
		Name: "described-params",
		ParamDefs: []core.ParamDef{
			{
				Name:        "notes",
				Type:        core.ParamDefTypeString,
				Description: "Free-form operator notes",
			},
		},
	})

	require.NotNil(t, details)
	require.NotNil(t, details.ParamDefs)
	require.Len(t, *details.ParamDefs, 1)
	require.NotNil(t, (*details.ParamDefs)[0].Description)
	assert.Equal(t, "Free-form operator notes", *(*details.ParamDefs)[0].Description)
}

func TestToNodeMapsStatuses(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		coreStatus  core.NodeStatus
		apiStatus   openapi.NodeStatus
		statusLabel openapi.NodeStatusLabel
	}{
		{
			name:        "running",
			coreStatus:  core.NodeRunning,
			apiStatus:   openapi.NodeStatusRunning,
			statusLabel: openapi.NodeStatusLabelRunning,
		},
		{
			name:        "retrying",
			coreStatus:  core.NodeRetrying,
			apiStatus:   openapi.NodeStatusRetrying,
			statusLabel: openapi.NodeStatusLabelRetrying,
		},
		{
			name:        "partial success",
			coreStatus:  core.NodePartiallySucceeded,
			apiStatus:   openapi.NodeStatusPartialSuccess,
			statusLabel: openapi.NodeStatusLabelPartiallySucceeded,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			node := &exec.Node{
				Status: tc.coreStatus,
				Step: core.Step{
					Name: "step-" + tc.name,
				},
			}

			converted := toNode(node)

			assert.Equal(t, tc.apiStatus, converted.Status)
			assert.Equal(t, tc.statusLabel, converted.StatusLabel)
		})
	}
}

func TestNodeStatusMappingIsExhaustive(t *testing.T) {
	t.Parallel()

	expected := map[openapi.NodeStatus]core.NodeStatus{
		openapi.NodeStatusNotStarted:     core.NodeNotStarted,
		openapi.NodeStatusRunning:        core.NodeRunning,
		openapi.NodeStatusFailed:         core.NodeFailed,
		openapi.NodeStatusAborted:        core.NodeAborted,
		openapi.NodeStatusSuccess:        core.NodeSucceeded,
		openapi.NodeStatusSkipped:        core.NodeSkipped,
		openapi.NodeStatusPartialSuccess: core.NodePartiallySucceeded,
		openapi.NodeStatusWaiting:        core.NodeWaiting,
		openapi.NodeStatusRejected:       core.NodeRejected,
		openapi.NodeStatusRetrying:       core.NodeRetrying,
	}

	assert.Len(t, nodeStatusMapping, len(expected))
	assert.Equal(t, expected, nodeStatusMapping)
}
