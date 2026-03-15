// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToDAGRunSummaryIncludesScheduleTime(t *testing.T) {
	status := exec.DAGRunStatus{
		Name:         "test-dag",
		DAGRunID:     "run-1",
		AutoRetryCount: 2,
		Status:       core.Queued,
		ScheduleTime: "2026-03-13T00:00:00Z",
	}

	summary := toDAGRunSummary(status)
	require.NotNil(t, summary.ScheduleTime)
	assert.Equal(t, status.ScheduleTime, *summary.ScheduleTime)
	assert.Equal(t, status.AutoRetryCount, summary.AutoRetryCount)
}

func TestToDAGRunDetailsIncludesScheduleTime(t *testing.T) {
	status := exec.DAGRunStatus{
		Name:         "test-dag",
		DAGRunID:     "run-1",
		AutoRetryCount: 3,
		Status:       core.Queued,
		QueuedAt:     "2026-03-13T00:01:00Z",
		ScheduleTime: "2026-03-13T00:00:00Z",
	}

	details := ToDAGRunDetails(status)
	require.NotNil(t, details.ScheduleTime)
	assert.Equal(t, status.ScheduleTime, *details.ScheduleTime)
	require.NotNil(t, details.QueuedAt)
	assert.Equal(t, status.QueuedAt, *details.QueuedAt)
	assert.Equal(t, status.AutoRetryCount, details.AutoRetryCount)
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
