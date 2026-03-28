// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package intg_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/persis/filewatermark"
	"github.com/dagu-org/dagu/internal/service/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOneOffScheduleRestartConsumesExistingRun(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dagsDir := filepath.Join(tmpDir, "dags")
	require.NoError(t, os.MkdirAll(dagsDir, 0755))

	scheduledAt := time.Date(2026, 3, 29, 2, 10, 0, 0, time.UTC)
	dagContent := fmt.Sprintf(`name: one-off-restart-test
schedule:
  start:
    - at: "%s"
steps:
  - name: step1
    command: echo "hello"
`, scheduledAt.Format(time.RFC3339))
	require.NoError(t, os.WriteFile(filepath.Join(dagsDir, "one-off-restart-test.yaml"), []byte(dagContent), 0644))

	th := test.SetupScheduler(t, test.WithDAGsDir(dagsDir))
	th.Config.Scheduler.RetryFailureWindow = 0

	dag, err := th.DAGStore.GetDetails(th.Context, "one-off-restart-test")
	require.NoError(t, err)
	require.Len(t, dag.Schedule, 1)

	watermarkStore := filewatermark.New(filepath.Join(th.Config.Paths.DataDir, "scheduler"))
	fingerprint := dag.Schedule[0].Fingerprint()
	runID := scheduler.GenerateOneOffRunID(dag.Name, fingerprint, scheduledAt)

	require.NoError(t, watermarkStore.Save(th.Context, &scheduler.SchedulerState{
		Version: scheduler.SchedulerStateVersion,
		DAGs: map[string]scheduler.DAGWatermark{
			dag.Name: {
				OneOffs: map[string]scheduler.OneOffScheduleState{
					fingerprint: {
						ScheduledTime: scheduledAt,
						Status:        scheduler.OneOffStatusPending,
					},
				},
			},
		},
	}))

	attempt, err := th.DAGRunStore.CreateAttempt(th.Context, dag, scheduledAt, runID, exec.NewDAGRunAttemptOptions{})
	require.NoError(t, err)
	initialStatus := exec.InitialStatus(dag)
	initialStatus.DAGRunID = runID
	initialStatus.AttemptID = attempt.ID()
	initialStatus.TriggerType = core.TriggerTypeScheduler
	initialStatus.ScheduleTime = scheduledAt.Format(time.RFC3339)
	require.NoError(t, attempt.Open(th.Context))
	require.NoError(t, attempt.Write(th.Context, initialStatus))
	require.NoError(t, attempt.Close(th.Context))

	sc, err := scheduler.New(
		th.Config,
		th.EntryReader,
		th.DAGRunMgr,
		th.DAGRunStore,
		th.QueueStore,
		th.ProcStore,
		th.ServiceRegistry,
		th.CoordinatorCli,
		watermarkStore,
	)
	require.NoError(t, err)
	sc.SetClock(func() time.Time { return scheduledAt })

	var dispatchCount atomic.Int32
	sc.SetDispatchFunc(func(context.Context, *core.DAG, string, core.TriggerType, time.Time) error {
		dispatchCount.Add(1)
		return nil
	})

	ctx, cancel := context.WithCancel(th.Context)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- sc.Start(ctx)
	}()

	require.Eventually(t, func() bool {
		state, err := watermarkStore.Load(th.Context)
		if err != nil {
			return false
		}
		entry, ok := state.DAGs[dag.Name]
		if !ok {
			return false
		}
		oneOff, ok := entry.OneOffs[fingerprint]
		return ok && oneOff.Status == scheduler.OneOffStatusConsumed
	}, 5*time.Second, 50*time.Millisecond)

	assert.Equal(t, int32(0), dispatchCount.Load())
	assert.Len(t, th.DAGRunStore.RecentAttempts(th.Context, dag.Name, 10), 1)

	sc.Stop(context.Background())
	cancel()

	select {
	case err := <-errCh:
		require.True(t,
			err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded),
			"unexpected scheduler shutdown error: %v", err,
		)
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler did not stop within 5 seconds")
	}
}
