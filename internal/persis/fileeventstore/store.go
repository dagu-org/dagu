// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

// Package fileeventstore provides a file-based implementation of the event store.
package fileeventstore

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

	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/service/eventstore"
	"github.com/google/uuid"
)

const (
	dirPermissions  = 0o750
	filePermissions = 0o640
	dayFormat       = "20060102"
	logSuffix       = ".jsonl"
	inboxSuffix     = ".json"
	logPrefix       = "_"
)

type Store struct {
	baseDir       string
	inboxDir      string
	quarantineDir string
}

var _ eventstore.Store = (*Store)(nil)

func New(baseDir string) (*Store, error) {
	if baseDir == "" {
		return nil, errors.New("fileeventstore: baseDir cannot be empty")
	}
	store := &Store{
		baseDir:       baseDir,
		inboxDir:      filepath.Join(baseDir, "inbox"),
		quarantineDir: filepath.Join(baseDir, "quarantine"),
	}
	for _, dir := range []string{store.baseDir, store.inboxDir, store.quarantineDir} {
		if err := os.MkdirAll(dir, dirPermissions); err != nil {
			return nil, fmt.Errorf("fileeventstore: create directory %s: %w", dir, err)
		}
	}
	return store, nil
}

func (s *Store) Emit(_ context.Context, event *eventstore.Event) error {
	if event == nil {
		return errors.New("fileeventstore: event cannot be nil")
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("fileeventstore: marshal event: %w", err)
	}
	recordedAt := event.RecordedAt
	if recordedAt.IsZero() {
		recordedAt = time.Now().UTC()
	}
	name := fmt.Sprintf("%020d-%s%s", recordedAt.UnixNano(), uuid.NewString(), inboxSuffix)
	path := filepath.Join(s.inboxDir, name)
	if err := fileutil.WriteFileAtomic(path, data, filePermissions); err != nil {
		return fmt.Errorf("fileeventstore: write inbox file: %w", err)
	}
	return nil
}

func (s *Store) Query(_ context.Context, filter eventstore.QueryFilter) (*eventstore.QueryResult, error) {
	files, err := s.listCommittedFiles(filter.StartTime, filter.EndTime)
	if err != nil {
		return nil, err
	}

	var entries []*eventstore.Event
	for _, file := range files {
		loaded, err := s.readCommittedEvents(file, filter)
		if err != nil {
			return nil, err
		}
		entries = append(entries, loaded...)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].OccurredAt.Equal(entries[j].OccurredAt) {
			return entries[i].RecordedAt.After(entries[j].RecordedAt)
		}
		return entries[i].OccurredAt.After(entries[j].OccurredAt)
	})

	total := len(entries)
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := max(filter.Offset, 0)
	if offset >= total {
		return &eventstore.QueryResult{Entries: []*eventstore.Event{}, Total: total}, nil
	}

	entries = entries[offset:]
	if limit < len(entries) {
		entries = entries[:limit]
	}

	return &eventstore.QueryResult{
		Entries: entries,
		Total:   total,
	}, nil
}

func (s *Store) listCommittedFiles(startTime, endTime time.Time) ([]string, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("fileeventstore: read directory %s: %w", s.baseDir, err)
	}

	startDay, hasStart := utcDay(startTime)
	endDay, hasEnd := utcDay(endTime)
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		day, ok := parseCommittedFileDay(entry.Name())
		if !ok {
			continue
		}
		if hasStart && day.Before(startDay) {
			continue
		}
		if hasEnd && day.After(endDay) {
			continue
		}
		files = append(files, filepath.Join(s.baseDir, entry.Name()))
	}

	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	return files, nil
}

func (s *Store) readCommittedEvents(filePath string, filter eventstore.QueryFilter) ([]*eventstore.Event, error) {
	f, err := os.Open(filePath) //nolint:gosec // controlled path
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("fileeventstore: open %s: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	var entries []*eventstore.Event
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		event := new(eventstore.Event)
		if err := json.Unmarshal(scanner.Bytes(), event); err != nil {
			slog.Warn("fileeventstore: skipping malformed event log line",
				slog.String("file", filePath),
				slog.Int("line", lineNum),
				slog.String("error", err.Error()))
			continue
		}
		event.Normalize()
		if err := event.Validate(); err != nil {
			slog.Warn("fileeventstore: skipping invalid event log line",
				slog.String("file", filePath),
				slog.Int("line", lineNum),
				slog.String("error", err.Error()))
			continue
		}
		if !matchesFilter(event, filter) {
			continue
		}
		entries = append(entries, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("fileeventstore: scan %s: %w", filePath, err)
	}
	return entries, nil
}

func matchesFilter(event *eventstore.Event, filter eventstore.QueryFilter) bool {
	if event == nil {
		return false
	}
	if filter.Kind != "" && event.Kind != filter.Kind {
		return false
	}
	if filter.Type != "" && event.Type != filter.Type {
		return false
	}
	if filter.DAGName != "" && event.DAGName != filter.DAGName {
		return false
	}
	if filter.DAGRunID != "" && event.DAGRunID != filter.DAGRunID {
		return false
	}
	if filter.AttemptID != "" && event.AttemptID != filter.AttemptID {
		return false
	}
	if filter.SessionID != "" && event.SessionID != filter.SessionID {
		return false
	}
	if filter.UserID != "" && event.UserID != filter.UserID {
		return false
	}
	if filter.Model != "" && event.Model != filter.Model {
		return false
	}
	if filter.Status != "" && event.Status != filter.Status {
		return false
	}
	if !filter.StartTime.IsZero() && event.OccurredAt.Before(filter.StartTime) {
		return false
	}
	if !filter.EndTime.IsZero() && event.OccurredAt.After(filter.EndTime) {
		return false
	}
	return true
}

func parseCommittedFileDay(name string) (time.Time, bool) {
	if !strings.HasPrefix(name, logPrefix) || !strings.HasSuffix(name, logSuffix) {
		return time.Time{}, false
	}
	datePart := strings.TrimSuffix(strings.TrimPrefix(name, logPrefix), logSuffix)
	day, err := time.Parse(dayFormat, datePart)
	if err != nil {
		return time.Time{}, false
	}
	return day.UTC(), true
}

func utcDay(t time.Time) (time.Time, bool) {
	if t.IsZero() {
		return time.Time{}, false
	}
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC), true
}
