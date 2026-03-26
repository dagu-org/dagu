// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package filedistributed

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/dirlock"
	"github.com/dagu-org/dagu/internal/core/exec"
)

type ActiveDistributedRunStore struct {
	baseDir string
}

func NewActiveDistributedRunStore(baseDir string) *ActiveDistributedRunStore {
	return &ActiveDistributedRunStore{baseDir: baseDir}
}

func (s *ActiveDistributedRunStore) activeRunsDir() string {
	return filepath.Join(s.baseDir, "active-runs")
}

func (s *ActiveDistributedRunStore) activeRunPath(attemptKey string) string {
	return filepath.Join(s.activeRunsDir(), encodeKey(attemptKey)+".json")
}

func (s *ActiveDistributedRunStore) activeRunLock(attemptKey string) dirlock.DirLock {
	return dirlock.New(filepath.Join(s.baseDir, "locks", "active-run-"+encodeKey(attemptKey)), &dirlock.LockOptions{
		StaleThreshold: 5 * time.Second,
		RetryInterval:  5 * time.Millisecond,
	})
}

func (s *ActiveDistributedRunStore) withActiveRunLock(ctx context.Context, attemptKey string, fn func() error) error {
	lockCtx := ctx
	if lockCtx == nil {
		lockCtx = context.Background()
	}
	lock := s.activeRunLock(attemptKey)
	if err := lock.Lock(lockCtx); err != nil {
		return fmt.Errorf("lock active distributed run %q: %w", attemptKey, err)
	}
	defer func() { _ = lock.Unlock() }()
	return fn()
}

func (s *ActiveDistributedRunStore) Upsert(ctx context.Context, record exec.ActiveDistributedRun) error {
	if record.AttemptKey == "" {
		return fmt.Errorf("attempt key is required")
	}

	return s.withActiveRunLock(ctx, record.AttemptKey, func() error {
		if record.UpdatedAt == 0 {
			record.UpdatedAt = time.Now().UTC().UnixMilli()
		}
		return writeJSONAtomic(s.activeRunPath(record.AttemptKey), record)
	})
}

func (s *ActiveDistributedRunStore) Delete(ctx context.Context, attemptKey string) error {
	if attemptKey == "" {
		return nil
	}
	return s.withActiveRunLock(ctx, attemptKey, func() error {
		err := os.Remove(s.activeRunPath(attemptKey))
		if err == nil || os.IsNotExist(err) {
			return nil
		}
		return err
	})
}

func (s *ActiveDistributedRunStore) Get(_ context.Context, attemptKey string) (*exec.ActiveDistributedRun, error) {
	var record exec.ActiveDistributedRun
	if err := readJSONFile(s.activeRunPath(attemptKey), &record); err != nil {
		if os.IsNotExist(err) {
			return nil, exec.ErrActiveRunNotFound
		}
		return nil, err
	}
	return &record, nil
}

func (s *ActiveDistributedRunStore) ListAll(_ context.Context) ([]exec.ActiveDistributedRun, error) {
	files, err := sortedFiles(s.activeRunsDir())
	if err != nil {
		return nil, err
	}

	records := make([]exec.ActiveDistributedRun, 0, len(files))
	for _, path := range files {
		var record exec.ActiveDistributedRun
		if err := readJSONFile(path, &record); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if record.AttemptKey == "" {
			continue
		}
		records = append(records, record)
	}
	return records, nil
}
