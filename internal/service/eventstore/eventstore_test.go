// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package eventstore

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPersistedDAGRunEventTypeForStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status core.Status
		want   EventType
		ok     bool
	}{
		{name: "NotStarted", status: core.NotStarted, ok: false},
		{name: "Queued", status: core.Queued, ok: false},
		{name: "Running", status: core.Running, ok: false},
		{name: "Rejected", status: core.Rejected, want: TypeDAGRunRejected, ok: true},
		{name: "Waiting", status: core.Waiting, want: TypeDAGRunWaiting, ok: true},
		{name: "Succeeded", status: core.Succeeded, want: TypeDAGRunSucceeded, ok: true},
		{name: "PartiallySucceeded", status: core.PartiallySucceeded, want: TypeDAGRunSucceeded, ok: true},
		{name: "Failed", status: core.Failed, want: TypeDAGRunFailed, ok: true},
		{name: "Aborted", status: core.Aborted, want: TypeDAGRunAborted, ok: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := PersistedDAGRunEventTypeForStatus(tt.status)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestServiceEmitDefaultsFieldsWithoutReadTimeRepair(t *testing.T) {
	t.Parallel()

	store := &captureStore{}
	service := New(store)

	event := &Event{
		ID:         "evt-1",
		OccurredAt: time.Date(2026, 4, 1, 10, 0, 0, 0, time.FixedZone("JST", 9*60*60)),
		Kind:       KindDAGRun,
		Type:       TypeDAGRunFailed,
	}

	require.NoError(t, service.Emit(context.Background(), event))
	require.NotNil(t, store.event)
	assert.Equal(t, SchemaVersion, store.event.SchemaVersion)
	assert.Equal(t, SourceServiceUnknown, store.event.SourceService)
	assert.False(t, store.event.RecordedAt.IsZero())
	assert.Equal(t, time.UTC, store.event.OccurredAt.Location())

	readEvent := &Event{}
	readEvent.Normalize()
	assert.Zero(t, readEvent.SchemaVersion)
	assert.Empty(t, readEvent.SourceService)
	assert.True(t, readEvent.RecordedAt.IsZero())
}

func TestStableIDUsesCollisionSafeFraming(t *testing.T) {
	t.Parallel()

	assert.NotEqual(t,
		stableID("a", "b\x1fc"),
		stableID("a\x1fb", "c"),
	)
}

func TestNewDAGRunEventEmbedsNotificationSnapshot(t *testing.T) {
	t.Parallel()

	status := &exec.DAGRunStatus{
		Name:       "briefing",
		DAGRunID:   "run-1",
		AttemptID:  "attempt-1",
		Status:     core.Failed,
		Error:      "boom",
		Log:        "/tmp/run.log",
		QueuedAt:   "2026-04-01T09:00:00Z",
		StartedAt:  "2026-04-01T09:01:00Z",
		FinishedAt: "2026-04-01T09:02:00Z",
		Nodes: []*exec.Node{
			{
				Step:   core.Step{Name: "fetch"},
				Status: core.NodeFailed,
				Error:  "node boom",
			},
		},
		OnFailure: &exec.Node{
			Step:  core.Step{Name: "notify"},
			Error: "handler boom",
		},
	}

	event := NewDAGRunEvent(Source{Service: SourceServiceServer, Instance: "test"}, TypeDAGRunFailed, status, map[string]any{"reason": "boom"})
	require.NotNil(t, event)
	require.NotNil(t, event.Data)
	assert.Equal(t, "boom", event.Data["reason"])

	restored, err := NotificationStatusFromEvent(event)
	require.NoError(t, err)
	require.NotNil(t, restored)
	assert.Equal(t, status.Name, restored.Name)
	assert.Equal(t, status.DAGRunID, restored.DAGRunID)
	assert.Equal(t, status.AttemptID, restored.AttemptID)
	assert.Equal(t, status.Status, restored.Status)
	assert.Equal(t, status.Error, restored.Error)
	assert.Equal(t, status.Log, restored.Log)
	assert.Equal(t, status.StartedAt, restored.StartedAt)
	assert.Equal(t, status.FinishedAt, restored.FinishedAt)
	require.Len(t, restored.Nodes, 1)
	assert.Equal(t, "fetch", restored.Nodes[0].Step.Name)
	assert.Equal(t, core.NodeFailed, restored.Nodes[0].Status)
	assert.Equal(t, "node boom", restored.Nodes[0].Error)
	require.NotNil(t, restored.OnFailure)
	assert.Equal(t, "notify", restored.OnFailure.Step.Name)
	assert.Equal(t, "handler boom", restored.OnFailure.Error)
}

type captureStore struct {
	event *Event
}

func (c *captureStore) Emit(_ context.Context, event *Event) error {
	c.event = event
	return nil
}

func (*captureStore) Query(context.Context, QueryFilter) (*QueryResult, error) {
	return nil, nil
}
