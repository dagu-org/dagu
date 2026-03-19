// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package chatbridge

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capturingNotificationStore struct {
	statuses []*exec.DAGRunStatus
	opts     []exec.ListDAGRunStatusesOptions
}

func (s *capturingNotificationStore) CreateAttempt(context.Context, *core.DAG, time.Time, string, exec.NewDAGRunAttemptOptions) (exec.DAGRunAttempt, error) {
	return nil, errors.New("unexpected call")
}

func (s *capturingNotificationStore) RecentAttempts(context.Context, string, int) []exec.DAGRunAttempt {
	return nil
}

func (s *capturingNotificationStore) LatestAttempt(context.Context, string) (exec.DAGRunAttempt, error) {
	return nil, errors.New("unexpected call")
}

func (s *capturingNotificationStore) ListStatuses(_ context.Context, opts ...exec.ListDAGRunStatusesOption) ([]*exec.DAGRunStatus, error) {
	var applied exec.ListDAGRunStatusesOptions
	for _, opt := range opts {
		opt(&applied)
	}
	s.opts = append(s.opts, applied)
	return s.statuses, nil
}

func (s *capturingNotificationStore) CompareAndSwapLatestAttemptStatus(context.Context, exec.DAGRunRef, string, core.Status, func(*exec.DAGRunStatus) error) (*exec.DAGRunStatus, bool, error) {
	return nil, false, errors.New("unexpected call")
}

func (s *capturingNotificationStore) FindAttempt(context.Context, exec.DAGRunRef) (exec.DAGRunAttempt, error) {
	return nil, errors.New("unexpected call")
}

func (s *capturingNotificationStore) FindSubAttempt(context.Context, exec.DAGRunRef, string) (exec.DAGRunAttempt, error) {
	return nil, errors.New("unexpected call")
}

func (s *capturingNotificationStore) CreateSubAttempt(context.Context, exec.DAGRunRef, string) (exec.DAGRunAttempt, error) {
	return nil, errors.New("unexpected call")
}

func (s *capturingNotificationStore) RemoveOldDAGRuns(context.Context, string, int, ...exec.RemoveOldDAGRunsOption) ([]string, error) {
	return nil, errors.New("unexpected call")
}

func (s *capturingNotificationStore) RenameDAGRuns(context.Context, string, string) error {
	return errors.New("unexpected call")
}

func (s *capturingNotificationStore) RemoveDAGRun(context.Context, exec.DAGRunRef) error {
	return errors.New("unexpected call")
}

func TestNotificationMonitor_SeedDeliveredUsesConfiguredLimit(t *testing.T) {
	t.Parallel()

	store := &capturingNotificationStore{
		statuses: []*exec.DAGRunStatus{
			{
				Name:      "briefing",
				Status:    core.Succeeded,
				DAGRunID:  "run-1",
				AttemptID: "attempt-1",
			},
		},
	}
	transport := &fakeNotificationTransport{destinations: []string{"dest-1"}}
	cfg := DefaultNotificationMonitorConfig()
	cfg.SeedLimit = 42

	monitor := NewNotificationMonitor(store, transport, slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)
	monitor.seedDelivered(context.Background())

	require.Len(t, store.opts, 1)
	assert.Equal(t, 42, store.opts[0].Limit)
	assert.True(t, monitor.IsDelivered("dest-1", store.statuses[0]))
}
