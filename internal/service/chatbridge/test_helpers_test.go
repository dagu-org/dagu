// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package chatbridge

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/service/eventstore"
)

type fakeNotificationTransport struct {
	destinations []string
	flushFn      func(context.Context, string, NotificationBatch, bool) bool
}

func (f *fakeNotificationTransport) NotificationDestinations() []string {
	return append([]string(nil), f.destinations...)
}

func (f *fakeNotificationTransport) FlushNotificationBatch(ctx context.Context, destination string, batch NotificationBatch, allowLLM bool) bool {
	if f.flushFn == nil {
		return true
	}
	return f.flushFn(ctx, destination, batch, allowLLM)
}

type stubNotificationStore struct {
	mu        sync.Mutex
	events    []*eventstore.Event
	failHead  bool
	readErr   error
	headCalls int
	readCalls int
}

var _ eventstore.Store = (*stubNotificationStore)(nil)
var _ eventstore.NotificationReader = (*stubNotificationStore)(nil)

func (s *stubNotificationStore) Emit(_ context.Context, event *eventstore.Event) error {
	if event == nil {
		return nil
	}
	event.Normalize()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

func (s *stubNotificationStore) Query(context.Context, eventstore.QueryFilter) (*eventstore.QueryResult, error) {
	return &eventstore.QueryResult{}, nil
}

func (s *stubNotificationStore) NotificationHeadCursor(context.Context) (eventstore.NotificationCursor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.headCalls++
	if s.failHead {
		return eventstore.NotificationCursor{}, errors.New("head unavailable")
	}
	return s.currentCursorLocked(), nil
}

func (s *stubNotificationStore) ReadNotificationEvents(_ context.Context, cursor eventstore.NotificationCursor) ([]*eventstore.Event, eventstore.NotificationCursor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.readCalls++
	if s.readErr != nil {
		return nil, cursor, s.readErr
	}

	index := int(cursor.Normalize().CommittedOffsets["events"])
	if index < 0 || index > len(s.events) {
		index = 0
	}
	events := append([]*eventstore.Event(nil), s.events[index:]...)
	return events, s.currentCursorLocked(), nil
}

func (s *stubNotificationStore) currentCursorLocked() eventstore.NotificationCursor {
	return eventstore.NotificationCursor{
		CommittedOffsets: map[string]int64{"events": int64(len(s.events))},
	}
}

func (s *stubNotificationStore) setHeadFailure(fail bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failHead = fail
}

func (s *stubNotificationStore) stats() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.headCalls, s.readCalls
}

func newTestNotificationMonitorConfig() NotificationMonitorConfig {
	cfg := DefaultNotificationMonitorConfig()
	cfg.PollInterval = 10 * time.Millisecond
	cfg.SuccessWindow = 10 * time.Millisecond
	cfg.UrgentWindow = 10 * time.Millisecond
	cfg.SeenEvictInterval = time.Hour
	return cfg
}

func requireNotificationStateWriteFailure(
	t *testing.T,
	store *notificationStateStore,
	state notificationMonitorState,
) {
	t.Helper()
	if store == nil {
		t.Fatal("notification state store is required")
	}
	if err := store.Save(context.Background(), state); err == nil {
		t.Skip("filesystem permissions do not block notification state writes in this environment")
	}
}
