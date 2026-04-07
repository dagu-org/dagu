// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package filedistributed

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dagucloud/dagu/internal/cmn/dirlock"
	"github.com/dagucloud/dagu/internal/core/exec"
)

type DAGRunLeaseStore struct {
	baseDir string
}

func NewDAGRunLeaseStore(baseDir string) *DAGRunLeaseStore {
	return &DAGRunLeaseStore{baseDir: baseDir}
}

func (s *DAGRunLeaseStore) leasesDir() string {
	return filepath.Join(s.baseDir, "leases")
}

func (s *DAGRunLeaseStore) leasePath(attemptKey string) string {
	return filepath.Join(s.leasesDir(), encodeKey(attemptKey)+".json")
}

func (s *DAGRunLeaseStore) leaseLock(attemptKey string) dirlock.DirLock {
	return dirlock.New(filepath.Join(s.baseDir, "locks", encodeKey(attemptKey)), &dirlock.LockOptions{
		StaleThreshold: 5 * time.Second,
		RetryInterval:  5 * time.Millisecond,
	})
}

func (s *DAGRunLeaseStore) withLeaseLock(ctx context.Context, attemptKey string, fn func() error) error {
	lockCtx := ctx
	if lockCtx == nil {
		lockCtx = context.Background()
	}
	lock := s.leaseLock(attemptKey)
	if err := lock.Lock(lockCtx); err != nil {
		return fmt.Errorf("lock lease %q: %w", attemptKey, err)
	}
	defer func() { _ = lock.Unlock() }()
	return fn()
}

func (s *DAGRunLeaseStore) Upsert(ctx context.Context, lease exec.DAGRunLease) error {
	if lease.AttemptKey == "" {
		return fmt.Errorf("attempt key is required")
	}

	return s.withLeaseLock(ctx, lease.AttemptKey, func() error {
		if lease.ClaimedAt == 0 {
			now := time.Now().UTC().UnixMilli()
			lease.ClaimedAt = now
			if lease.LastHeartbeatAt == 0 {
				lease.LastHeartbeatAt = now
			}
		}
		if lease.LastHeartbeatAt == 0 {
			lease.LastHeartbeatAt = time.Now().UTC().UnixMilli()
		}
		return writeJSONAtomic(s.leasePath(lease.AttemptKey), lease)
	})
}

func (s *DAGRunLeaseStore) Touch(ctx context.Context, attemptKey string, observedAt time.Time) error {
	return s.withLeaseLock(ctx, attemptKey, func() error {
		lease, err := s.Get(ctx, attemptKey)
		if err != nil {
			return err
		}
		lease.LastHeartbeatAt = observedAt.UTC().UnixMilli()
		return writeJSONAtomic(s.leasePath(attemptKey), lease)
	})
}

func (s *DAGRunLeaseStore) Delete(ctx context.Context, attemptKey string) error {
	return s.withLeaseLock(ctx, attemptKey, func() error {
		err := os.Remove(s.leasePath(attemptKey))
		if err == nil || os.IsNotExist(err) {
			return nil
		}
		return err
	})
}

func (s *DAGRunLeaseStore) Get(_ context.Context, attemptKey string) (*exec.DAGRunLease, error) {
	var lease exec.DAGRunLease
	if err := readJSONFile(s.leasePath(attemptKey), &lease); err != nil {
		if os.IsNotExist(err) {
			return nil, exec.ErrDAGRunLeaseNotFound
		}
		return nil, err
	}
	return &lease, nil
}

func (s *DAGRunLeaseStore) ListByQueue(ctx context.Context, queueName string) ([]exec.DAGRunLease, error) {
	leases, err := s.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	filtered := make([]exec.DAGRunLease, 0, len(leases))
	for _, lease := range leases {
		if lease.QueueName == queueName {
			filtered = append(filtered, lease)
		}
	}
	return filtered, nil
}

func (s *DAGRunLeaseStore) ListAll(_ context.Context) ([]exec.DAGRunLease, error) {
	files, err := sortedFiles(s.leasesDir())
	if err != nil {
		return nil, err
	}

	leases := make([]exec.DAGRunLease, 0, len(files))
	for _, path := range files {
		var lease exec.DAGRunLease
		if err := readJSONFile(path, &lease); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if lease.AttemptKey == "" {
			continue
		}
		leases = append(leases, lease)
	}
	return leases, nil
}
