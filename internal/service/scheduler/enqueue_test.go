// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/service/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnqueueCatchupRun_PersistsQueuedCatchupMetadata(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)
	dag := th.DAG(t, `name: enqueue-catchup-dag
steps:
  - name: step1
    command: echo enqueue
`)

	runID := "catchup-run-1"
	scheduleTime := time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)

	err := scheduler.EnqueueCatchupRun(
		th.Context,
		th.DAGRunStore,
		th.QueueStore,
		th.Config.Paths.LogDir,
		dag.DAG,
		runID,
		core.TriggerTypeCatchUp,
		scheduleTime,
	)
	require.NoError(t, err)

	attempt, err := th.DAGRunStore.FindAttempt(th.Context, exec.NewDAGRunRef(dag.Name, runID))
	require.NoError(t, err)

	status, err := attempt.ReadStatus(th.Context)
	require.NoError(t, err)

	require.Equal(t, core.Queued, status.Status)
	require.Equal(t, core.TriggerTypeCatchUp, status.TriggerType)
	require.Equal(t, stringutil.FormatTime(scheduleTime), status.ScheduleTime)
	require.NotEmpty(t, status.Log)
	assert.Contains(t, status.Log, filepath.Join(th.Config.Paths.LogDir, dag.Name))

	items, err := th.QueueStore.List(th.Context, dag.ProcGroup())
	require.NoError(t, err)
	require.Len(t, items, 1)

	ref, err := items[0].Data()
	require.NoError(t, err)
	assert.Equal(t, exec.NewDAGRunRef(dag.Name, runID), *ref)
}
