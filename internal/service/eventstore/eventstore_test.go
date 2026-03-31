// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package eventstore

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
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
