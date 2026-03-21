// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package fileproc

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/backoff"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core/exec"
)

var _ exec.ProcStore = (*Store)(nil)

// Store is a struct that implements the ProcStore interface.
type Store struct {
	baseDir           string
	staleTime         time.Duration
	heartbeatInterval time.Duration
	syncInterval      time.Duration
	procGroups        sync.Map
}

// StoreOption configures a Store.
type StoreOption func(*Store)

// WithStaleThreshold sets the duration after which a proc file is considered stale.
func WithStaleThreshold(d time.Duration) StoreOption {
	return func(s *Store) {
		if d > 0 {
			s.staleTime = d
		}
	}
}

// WithHeartbeatInterval sets the heartbeat write interval.
func WithHeartbeatInterval(d time.Duration) StoreOption {
	return func(s *Store) {
		if d > 0 {
			s.heartbeatInterval = d
		}
	}
}

// WithHeartbeatSyncInterval sets the heartbeat fsync interval.
func WithHeartbeatSyncInterval(d time.Duration) StoreOption {
	return func(s *Store) {
		if d > 0 {
			s.syncInterval = d
		}
	}
}

// New creates a new instance of Store with the specified base directory.
func New(baseDir string, opts ...StoreOption) *Store {
	s := &Store{
		baseDir:           baseDir,
		staleTime:         90 * time.Second,
		heartbeatInterval: 5 * time.Second,
		syncInterval:      10 * time.Second,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Lock locks process group
func (s *Store) Lock(ctx context.Context, groupName string) error {
	basePolicy := backoff.NewExponentialBackoffPolicy(500 * time.Millisecond)
	basePolicy.BackoffFactor = 2.0
	basePolicy.MaxInterval = time.Second * 60
	basePolicy.MaxRetries = 10

	policy := backoff.WithJitter(basePolicy, backoff.Jitter)
	return backoff.Retry(ctx, func(_ context.Context) error {
		procGroup := s.newProcGroup(groupName)
		return procGroup.TryLock()
	}, policy, func(_ error) bool {
		return ctx.Err() == nil
	})
}

// Lock locks process group
func (s *Store) Unlock(ctx context.Context, groupName string) {
	procGroup := s.newProcGroup(groupName)
	if err := procGroup.Unlock(); err != nil {
		logger.Error(ctx, "Failed to unlock the proc group",
			tag.Error(err))
	}
}

// CountAlive implements models.ProcStore.
func (s *Store) CountAlive(ctx context.Context, groupName string) (int, error) {
	procGroup := s.newProcGroup(groupName)
	return procGroup.Count(ctx)
}

func (s *Store) CountAliveByDAGName(ctx context.Context, groupName, dagName string) (int, error) {
	procGroup := s.newProcGroup(groupName)
	return procGroup.CountByDAGName(ctx, dagName)
}

// ListAlive implements models.ProcStore.
func (s *Store) ListAlive(ctx context.Context, groupName string) ([]exec.DAGRunRef, error) {
	procGroup := s.newProcGroup(groupName)
	return procGroup.ListAlive(ctx)
}

// Acquire implements models.ProcStore.
func (s *Store) Acquire(ctx context.Context, groupName string, meta exec.ProcMeta) (exec.ProcHandle, error) {
	procGroup := s.newProcGroup(groupName)
	proc, err := procGroup.Acquire(ctx, meta)
	if err != nil {
		return nil, err
	}
	if err := proc.startHeartbeat(ctx); err != nil {
		return nil, err
	}
	return proc, nil
}

// IsRunAlive implements models.ProcStore.
func (s *Store) IsRunAlive(ctx context.Context, groupName string, dagRun exec.DAGRunRef) (bool, error) {
	procGroup := s.newProcGroup(groupName)
	return procGroup.IsRunAlive(ctx, dagRun)
}

// ListAllAlive implements models.ProcStore.
// Returns all running DAG runs across all process groups.
func (s *Store) ListAllAlive(ctx context.Context) (map[string][]exec.DAGRunRef, error) {
	result := make(map[string][]exec.DAGRunRef)

	entries, err := s.ListAllEntries(ctx)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.Fresh {
			continue
		}
		result[entry.GroupName] = append(result[entry.GroupName], entry.Meta.DAGRun())
	}
	for groupName := range result {
		deduped := freshRefs(entriesForGroup(entries, groupName))
		if len(deduped) == 0 {
			delete(result, groupName)
			continue
		}
		result[groupName] = deduped
	}
	return result, nil
}

// ListEntries implements exec.ProcStore.
func (s *Store) ListEntries(ctx context.Context, groupName string) ([]exec.ProcEntry, error) {
	procGroup := s.newProcGroup(groupName)
	return procGroup.ListEntries(ctx)
}

// ListAllEntries implements exec.ProcStore.
func (s *Store) ListAllEntries(ctx context.Context) ([]exec.ProcEntry, error) {
	// Create base directory if it doesn't exist
	if _, err := os.Stat(s.baseDir); os.IsNotExist(err) {
		return []exec.ProcEntry{}, nil
	}

	// Read all directories in the base directory - each directory is a process group
	dirEntries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, err
	}

	groupNames := make([]string, 0, len(dirEntries))
	for _, entry := range dirEntries {
		if !entry.IsDir() {
			continue
		}
		groupNames = append(groupNames, entry.Name())
	}
	sort.Strings(groupNames)

	var result []exec.ProcEntry
	for _, groupName := range groupNames {
		procGroup := s.newProcGroup(groupName)
		entries, err := procGroup.ListEntries(ctx)
		if err != nil {
			return nil, err
		}
		result = append(result, entries...)
	}

	return result, nil
}

// RemoveIfStale implements exec.ProcStore.
func (s *Store) RemoveIfStale(ctx context.Context, entry exec.ProcEntry) error {
	if entry.GroupName == "" {
		return nil
	}
	procGroup := s.newProcGroup(entry.GroupName)
	return procGroup.RemoveIfStale(ctx, entry)
}

// Validate fails if any proc entry in the store is invalid or uses an unsupported format.
func (s *Store) Validate(ctx context.Context) error {
	_, err := s.ListAllEntries(ctx)
	return err
}

func entriesForGroup(entries []exec.ProcEntry, groupName string) []exec.ProcEntry {
	filtered := make([]exec.ProcEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.GroupName == groupName {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func (s *Store) newProcGroup(groupName string) *ProcGroup {
	// Check if the ProcGroup already exists
	if pg, ok := s.procGroups.Load(groupName); ok {
		return pg.(*ProcGroup)
	}

	// Create a new ProcGroup only if it doesn't exist
	pgBaseDir := filepath.Join(s.baseDir, groupName)
	newPG := NewProcGroup(pgBaseDir, groupName, s.staleTime, s.heartbeatInterval, s.syncInterval)
	pg, _ := s.procGroups.LoadOrStore(groupName, newPG)
	return pg.(*ProcGroup)
}
