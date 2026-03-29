// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package fileeventfeed

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/service/eventfeed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreAppendConcurrentAcrossInstances(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	storeA, err := New(baseDir, 0)
	require.NoError(t, err)
	storeB, err := New(baseDir, 0)
	require.NoError(t, err)

	const total = 24
	var wg sync.WaitGroup
	errCh := make(chan error, total)
	for i := range total {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			entry := &eventfeed.Entry{
				ID:        "event-" + strconv.Itoa(i),
				Timestamp: time.Date(2026, 3, 29, 12, i, 0, 0, time.UTC),
				Type:      eventfeed.EventTypeFailed,
				DAGName:   "test-dag",
				DAGRunID:  "run-" + strconv.Itoa(i),
			}

			var appendErr error
			if i%2 == 0 {
				appendErr = storeA.Append(context.Background(), entry)
			} else {
				appendErr = storeB.Append(context.Background(), entry)
			}
			errCh <- appendErr
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	result, err := storeA.Query(context.Background(), eventfeed.QueryFilter{})
	require.NoError(t, err)
	require.Equal(t, total, result.Total)
	require.Len(t, result.Entries, total)

	seen := map[string]struct{}{}
	for _, entry := range result.Entries {
		seen[entry.ID] = struct{}{}
	}
	require.Len(t, seen, total)
}

func TestStoreQueryReturnsNewestFirstAcrossShards(t *testing.T) {
	t.Parallel()

	store, err := New(t.TempDir(), 0)
	require.NoError(t, err)

	entries := []eventfeed.Entry{
		{ID: "old", Timestamp: time.Date(2026, 3, 27, 8, 0, 0, 0, time.UTC), Type: eventfeed.EventTypeWaiting, DAGName: "test", DAGRunID: "run-old"},
		{ID: "newer", Timestamp: time.Date(2026, 3, 28, 21, 0, 0, 0, time.UTC), Type: eventfeed.EventTypeFailed, DAGName: "test", DAGRunID: "run-newer"},
		{ID: "newest", Timestamp: time.Date(2026, 3, 29, 9, 0, 0, 0, time.UTC), Type: eventfeed.EventTypeAborted, DAGName: "test", DAGRunID: "run-newest"},
	}
	for _, entry := range entries {
		require.NoError(t, store.Append(context.Background(), &entry))
	}

	result, err := store.Query(context.Background(), eventfeed.QueryFilter{})
	require.NoError(t, err)
	require.Equal(t, []string{"newest", "newer", "old"}, entryIDs(result.Entries))
}

func TestStoreQueryToleratesMalformedTrailingLine(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store, err := New(baseDir, 0)
	require.NoError(t, err)

	dayDir := filepath.Join(baseDir, "2026-03-29")
	require.NoError(t, os.MkdirAll(dayDir, dirPermissions))

	valid := eventfeed.Entry{
		ID:        "good",
		Timestamp: time.Date(2026, 3, 29, 10, 0, 0, 0, time.UTC),
		Type:      eventfeed.EventTypeWaiting,
		DAGName:   "test",
		DAGRunID:  "run-1",
	}
	data, err := json.Marshal(valid)
	require.NoError(t, err)

	filePath := filepath.Join(dayDir, entriesFileName)
	require.NoError(t, os.WriteFile(filePath, append(append(data, '\n'), []byte("{bad json")...), filePermissions))

	result, err := store.Query(context.Background(), eventfeed.QueryFilter{})
	require.NoError(t, err)
	require.Equal(t, 1, result.Total)
	require.Len(t, result.Entries, 1)
	require.Equal(t, "good", result.Entries[0].ID)
}

func TestStoreRetentionCleanupDeletesExpiredShards(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store, err := New(baseDir, 30)
	require.NoError(t, err)

	expiredDir := filepath.Join(baseDir, "2026-01-01")
	recentDir := filepath.Join(baseDir, "2026-03-10")
	ignoredDir := filepath.Join(baseDir, "notes")
	require.NoError(t, os.MkdirAll(expiredDir, dirPermissions))
	require.NoError(t, os.MkdirAll(recentDir, dirPermissions))
	require.NoError(t, os.MkdirAll(ignoredDir, dirPermissions))

	require.NoError(t, store.purgeExpiredShards(time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)))

	_, expiredErr := os.Stat(expiredDir)
	assert.True(t, os.IsNotExist(expiredErr))

	_, recentErr := os.Stat(recentDir)
	require.NoError(t, recentErr)

	_, ignoredErr := os.Stat(ignoredDir)
	require.NoError(t, ignoredErr)
}

func entryIDs(entries []eventfeed.Entry) []string {
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.ID)
	}
	return ids
}
