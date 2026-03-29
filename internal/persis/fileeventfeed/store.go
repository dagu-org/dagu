// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package fileeventfeed

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/dirlock"
	"github.com/dagu-org/dagu/internal/service/eventfeed"
)

const (
	dayLayout        = "2006-01-02"
	entriesFileName  = "entries.jsonl"
	dirPermissions   = 0750
	filePermissions  = 0640
	maxScanTokenSize = 1024 * 1024
)

// Store persists event-feed entries as daily JSONL shards.
type Store struct {
	baseDir       string
	retentionDays int
	cleaner       *cleaner
}

var _ eventfeed.Store = (*Store)(nil)

// New constructs a file-backed event-feed store.
func New(baseDir string, retentionDays int) (*Store, error) {
	if baseDir == "" {
		return nil, errors.New("fileeventfeed: baseDir cannot be empty")
	}
	if err := os.MkdirAll(baseDir, dirPermissions); err != nil {
		return nil, fmt.Errorf("fileeventfeed: create base directory: %w", err)
	}
	store := &Store{
		baseDir:       baseDir,
		retentionDays: retentionDays,
	}
	if retentionDays > 0 {
		store.cleaner = newCleaner(store)
	}
	return store, nil
}

// Close stops background cleanup.
func (s *Store) Close() error {
	if s.cleaner != nil {
		s.cleaner.stop()
	}
	return nil
}

// Append writes a single event-feed entry to the appropriate daily shard.
func (s *Store) Append(ctx context.Context, entry *eventfeed.Entry) error {
	if entry == nil {
		return errors.New("fileeventfeed: entry cannot be nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	dayDir := filepath.Join(s.baseDir, entry.Timestamp.UTC().Format(dayLayout))
	if err := os.MkdirAll(dayDir, dirPermissions); err != nil {
		return fmt.Errorf("fileeventfeed: create day directory: %w", err)
	}

	lock := dirlock.New(dayDir, &dirlock.LockOptions{
		RetryInterval: 10 * time.Millisecond,
	})
	if err := lock.Lock(ctx); err != nil {
		return fmt.Errorf("fileeventfeed: lock day shard: %w", err)
	}
	defer func() { _ = lock.Unlock() }()

	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("fileeventfeed: marshal entry: %w", err)
	}

	filePath := filepath.Join(dayDir, entriesFileName)
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, filePermissions) //nolint:gosec
	if err != nil {
		return fmt.Errorf("fileeventfeed: open shard file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("fileeventfeed: append entry: %w", err)
	}
	return nil
}

// Query reads event-feed entries across shards newest-first.
func (s *Store) Query(ctx context.Context, filter eventfeed.QueryFilter) (*eventfeed.QueryResult, error) {
	days, err := s.listDayShards()
	if err != nil {
		return nil, err
	}

	matched := make([]eventfeed.Entry, 0)
	for _, day := range days {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		dayTime, parseErr := time.Parse(dayLayout, filepath.Base(day))
		if parseErr != nil {
			continue
		}
		if !dayInRange(dayTime, filter.StartTime, filter.EndTime) {
			continue
		}

		filePath := filepath.Join(day, entriesFileName)
		entries, readErr := s.readEntries(ctx, filePath, filter)
		if readErr != nil {
			if errors.Is(readErr, os.ErrNotExist) {
				continue
			}
			return nil, readErr
		}
		matched = append(matched, entries...)
	}

	sort.Slice(matched, func(i, j int) bool {
		if matched[i].Timestamp.Equal(matched[j].Timestamp) {
			return matched[i].ID > matched[j].ID
		}
		return matched[i].Timestamp.After(matched[j].Timestamp)
	})

	total := len(matched)
	offset := max(filter.Offset, 0)
	if offset >= total {
		return &eventfeed.QueryResult{Entries: []eventfeed.Entry{}, Total: total}, nil
	}
	matched = matched[offset:]

	limit := filter.Limit
	if limit > 0 && limit < len(matched) {
		matched = matched[:limit]
	}

	return &eventfeed.QueryResult{
		Entries: matched,
		Total:   total,
	}, nil
}

func (s *Store) listDayShards() ([]string, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("fileeventfeed: read base directory: %w", err)
	}

	dirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := time.Parse(dayLayout, entry.Name()); err != nil {
			continue
		}
		dirs = append(dirs, filepath.Join(s.baseDir, entry.Name()))
	}

	sort.Slice(dirs, func(i, j int) bool {
		return filepath.Base(dirs[i]) > filepath.Base(dirs[j])
	})
	return dirs, nil
}

func (s *Store) readEntries(ctx context.Context, filePath string, filter eventfeed.QueryFilter) ([]eventfeed.Entry, error) {
	f, err := os.Open(filePath) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), maxScanTokenSize)

	matched := make([]eventfeed.Entry, 0)
	lineNum := 0
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry eventfeed.Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			slog.Warn("fileeventfeed: skipping malformed entry",
				slog.String("file", filePath),
				slog.Int("line", lineNum),
				slog.String("error", err.Error()),
			)
			continue
		}

		if !matchesFilter(entry, filter) {
			continue
		}
		matched = append(matched, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("fileeventfeed: scan shard: %w", err)
	}
	return matched, nil
}

func matchesFilter(entry eventfeed.Entry, filter eventfeed.QueryFilter) bool {
	if filter.Type != "" && entry.Type != filter.Type {
		return false
	}
	if !filter.StartTime.IsZero() && entry.Timestamp.Before(filter.StartTime) {
		return false
	}
	if !filter.EndTime.IsZero() && entry.Timestamp.After(filter.EndTime) {
		return false
	}
	if filter.DAGName != "" && !containsFold(entry.DAGName, filter.DAGName) {
		return false
	}
	if filter.DAGRunID != "" && !containsFold(entry.DAGRunID, filter.DAGRunID) && !containsFold(entry.SubDAGRunID, filter.DAGRunID) {
		return false
	}
	if filter.Actor != "" && !containsFold(entry.Actor, filter.Actor) {
		return false
	}
	if filter.Search != "" {
		search := filter.Search
		if !containsFold(entry.DAGName, search) &&
			!containsFold(entry.StepName, search) &&
			!containsFold(entry.Actor, search) &&
			!containsFold(entry.Reason, search) &&
			!containsFold(entry.ResultingRunStatus, search) &&
			!containsFold(string(entry.Type), search) &&
			!containsFold(entry.DAGRunID, search) &&
			!containsFold(entry.SubDAGRunID, search) {
			return false
		}
	}
	return true
}

func containsFold(value, search string) bool {
	if search == "" {
		return true
	}
	return strings.Contains(strings.ToLower(value), strings.ToLower(search))
}

func dayInRange(day time.Time, start, end time.Time) bool {
	dayStart := day.UTC()
	dayEnd := dayStart.Add(24*time.Hour - time.Nanosecond)

	if !start.IsZero() && dayEnd.Before(start.UTC()) {
		return false
	}
	if !end.IsZero() && dayStart.After(end.UTC()) {
		return false
	}
	return true
}

func (s *Store) purgeExpiredShards(now time.Time) error {
	if s.retentionDays <= 0 {
		return nil
	}

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("fileeventfeed: read base directory for cleanup: %w", err)
	}

	cutoff := time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day(), 0, 0, 0, 0, time.UTC).
		AddDate(0, 0, -s.retentionDays)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		day, err := time.Parse(dayLayout, entry.Name())
		if err != nil {
			continue
		}
		if !day.Before(cutoff) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(s.baseDir, entry.Name())); err != nil {
			return fmt.Errorf("fileeventfeed: remove expired shard %s: %w", entry.Name(), err)
		}
	}
	return nil
}
