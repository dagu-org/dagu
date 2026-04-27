// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package filedistributed

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dagucloud/dagu/internal/core/exec"
)

type WorkerHeartbeatStore struct {
	baseDir string
}

func NewWorkerHeartbeatStore(baseDir string) *WorkerHeartbeatStore {
	return &WorkerHeartbeatStore{baseDir: baseDir}
}

func (s *WorkerHeartbeatStore) recordPath(workerID string) string {
	return filepath.Join(s.baseDir, "workers", encodeKey(workerID)+".json")
}

func (s *WorkerHeartbeatStore) Upsert(_ context.Context, record exec.WorkerHeartbeatRecord) error {
	if record.WorkerID == "" {
		return fmt.Errorf("worker id is required")
	}
	if record.LastHeartbeatAt == 0 {
		record.LastHeartbeatAt = time.Now().UTC().UnixMilli()
	}
	return writeJSONAtomic(s.recordPath(record.WorkerID), record)
}

func (s *WorkerHeartbeatStore) Get(_ context.Context, workerID string) (*exec.WorkerHeartbeatRecord, error) {
	if workerID == "" {
		return nil, exec.ErrWorkerHeartbeatNotFound
	}

	var record exec.WorkerHeartbeatRecord
	if err := readJSONFile(s.recordPath(workerID), &record); err != nil {
		if os.IsNotExist(err) {
			return nil, exec.ErrWorkerHeartbeatNotFound
		}
		return nil, err
	}
	if record.WorkerID == "" {
		return nil, exec.ErrWorkerHeartbeatNotFound
	}
	return &record, nil
}

func (s *WorkerHeartbeatStore) List(_ context.Context) ([]exec.WorkerHeartbeatRecord, error) {
	files, err := sortedFiles(filepath.Join(s.baseDir, "workers"))
	if err != nil {
		return nil, err
	}

	records := make([]exec.WorkerHeartbeatRecord, 0, len(files))
	for _, path := range files {
		var record exec.WorkerHeartbeatRecord
		if err := readJSONFile(path, &record); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if record.WorkerID == "" {
			continue
		}
		records = append(records, record)
	}

	return records, nil
}

func (s *WorkerHeartbeatStore) DeleteStale(_ context.Context, before time.Time) (int, error) {
	records, err := s.List(context.Background())
	if err != nil {
		return 0, err
	}

	removed := 0
	for _, record := range records {
		if record.LastHeartbeatTime().After(before) {
			continue
		}
		if err := removeFile(s.recordPath(record.WorkerID)); err != nil && !os.IsNotExist(err) {
			return removed, err
		}
		removed++
	}

	return removed, nil
}
