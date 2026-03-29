// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package eventfeed

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServiceRecordAssignsIDAndTimestamp(t *testing.T) {
	t.Parallel()

	store := &capturingStore{}
	svc := New(store)

	err := svc.Record(context.Background(), Entry{Type: EventTypeFailed, DAGName: "test", DAGRunID: "run-1"})
	require.NoError(t, err)

	recorded := store.last()
	require.NotNil(t, recorded)
	require.NotEmpty(t, recorded.ID)
	require.False(t, recorded.Timestamp.IsZero())
	require.Equal(t, EventTypeFailed, recorded.Type)
	require.Equal(t, "test", recorded.DAGName)
}

func TestServiceRecordHonorsWriteTimeout(t *testing.T) {
	t.Parallel()

	svc := New(&blockingAppendStore{}, WithWriteTimeout(10*time.Millisecond))
	start := time.Now()
	err := svc.Record(context.Background(), Entry{Type: EventTypeWaiting, DAGName: "test", DAGRunID: "run-1"})
	require.Error(t, err)
	require.Less(t, time.Since(start), 250*time.Millisecond)
}

type capturingStore struct {
	mu      sync.Mutex
	entries []Entry
}

func (s *capturingStore) Append(_ context.Context, entry *Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, *entry)
	return nil
}

func (s *capturingStore) Query(context.Context, QueryFilter) (*QueryResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Entry, len(s.entries))
	copy(out, s.entries)
	return &QueryResult{Entries: out, Total: len(out)}, nil
}

func (s *capturingStore) Close() error { return nil }

func (s *capturingStore) last() *Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.entries) == 0 {
		return nil
	}
	entry := s.entries[len(s.entries)-1]
	return &entry
}

type blockingAppendStore struct{}

func (b *blockingAppendStore) Append(ctx context.Context, _ *Entry) error {
	<-ctx.Done()
	return ctx.Err()
}

func (b *blockingAppendStore) Query(context.Context, QueryFilter) (*QueryResult, error) {
	return &QueryResult{}, nil
}

func (b *blockingAppendStore) Close() error { return nil }
