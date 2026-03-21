// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package fileproc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/dirlock"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core/exec"
)

// ProcGroup is a struct that manages process files for a given process group.
type ProcGroup struct {
	dirlock.DirLock

	groupName         string
	baseDir           string
	staleTime         time.Duration
	heartbeatInterval time.Duration
	syncInterval      time.Duration
	mu                sync.Mutex
}

// NewProcGroup creates a new instance of a ProcGroup with the specified base directory and group name.
func NewProcGroup(baseDir, groupName string, staleTime, heartbeatInterval, syncInterval time.Duration) *ProcGroup {
	dirLock := dirlock.New(baseDir, &dirlock.LockOptions{
		StaleThreshold: 5 * time.Second,
		RetryInterval:  100 * time.Millisecond,
	})
	return &ProcGroup{
		DirLock:           dirLock,
		baseDir:           baseDir,
		groupName:         groupName,
		staleTime:         staleTime,
		heartbeatInterval: heartbeatInterval,
		syncInterval:      syncInterval,
	}
}

func (pg *ProcGroup) CountByDAGName(ctx context.Context, dagName string) (int, error) {
	entries, err := pg.ListEntries(ctx)
	if err != nil {
		return 0, err
	}
	seen := make(map[string]struct{})
	for _, entry := range entries {
		if !entry.Fresh || entry.Meta.Name != dagName {
			continue
		}
		seen[entry.Meta.DAGRun().String()] = struct{}{}
	}
	return len(seen), nil
}

// Count retrieves the count of alive proc entries for the specified group.
func (pg *ProcGroup) Count(ctx context.Context) (int, error) {
	entries, err := pg.ListEntries(ctx)
	if err != nil {
		return 0, err
	}
	seen := make(map[string]struct{})
	for _, entry := range entries {
		if !entry.Fresh {
			continue
		}
		seen[entry.Meta.DAGRun().String()] = struct{}{}
	}
	return len(seen), nil
}

// Acquire creates a proc heartbeat for the specified execution metadata.
func (pg *ProcGroup) Acquire(_ context.Context, meta exec.ProcMeta) (*ProcHandle, error) {
	if meta.StartedAt <= 0 {
		meta.StartedAt = time.Now().Unix()
	}
	if err := validateProcMeta(meta); err != nil {
		return nil, err
	}
	fileName := procFilePath(pg.baseDir, exec.NewUTC(time.Now()), meta)
	return NewProcHandler(fileName, meta, pg.heartbeatInterval, pg.syncInterval), nil
}

// IsRunAlive checks if a specific DAG run has a fresh proc heartbeat entry.
func (pg *ProcGroup) IsRunAlive(ctx context.Context, dagRun exec.DAGRunRef) (bool, error) {
	entries, err := pg.ListEntries(ctx)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry.Fresh && entry.Meta.Name == dagRun.Name && entry.Meta.DAGRunID == dagRun.ID {
			return true, nil
		}
	}
	return false, nil
}

// ListAlive returns a list of alive DAG runs by scanning proc entries.
func (pg *ProcGroup) ListAlive(ctx context.Context) ([]exec.DAGRunRef, error) {
	entries, err := pg.ListEntries(ctx)
	if err != nil {
		return nil, err
	}
	refs := freshRefs(entries)
	return refs, nil
}

// ListEntries returns all proc entries for the group, including stale entries.
func (pg *ProcGroup) ListEntries(ctx context.Context) ([]exec.ProcEntry, error) {
	pg.mu.Lock()
	defer pg.mu.Unlock()

	entries, err := pg.listEntriesLocked(ctx)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// RemoveIfStale removes the exact proc file if it is still stale and unchanged.
func (pg *ProcGroup) RemoveIfStale(ctx context.Context, entry exec.ProcEntry) error {
	pg.mu.Lock()
	defer pg.mu.Unlock()

	current, err := readProcEntry(entry.FilePath, pg.groupName, pg.staleTime, time.Now())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if current.Fresh || !sameProcEntry(current, entry) {
		return nil
	}
	if err := os.Remove(entry.FilePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_ = os.Remove(filepath.Dir(entry.FilePath))
	logger.Info(ctx, "Removed stale proc file", tag.File(entry.FilePath))
	return nil
}

func (pg *ProcGroup) listEntriesLocked(_ context.Context) ([]exec.ProcEntry, error) {
	if _, err := os.Stat(pg.baseDir); errors.Is(err, os.ErrNotExist) {
		return []exec.ProcEntry{}, nil
	}

	dagEntries, err := os.ReadDir(pg.baseDir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, dagEntry := range dagEntries {
		if !dagEntry.IsDir() || dagEntry.Name() == "" || dagEntry.Name()[0] == '.' {
			continue
		}
		dagDir := filepath.Join(pg.baseDir, dagEntry.Name())
		procEntries, err := os.ReadDir(dagDir)
		if err != nil {
			return nil, err
		}
		for _, procEntry := range procEntries {
			if procEntry.IsDir() || filepath.Ext(procEntry.Name()) != procFileExt {
				continue
			}
			files = append(files, filepath.Join(dagDir, procEntry.Name()))
		}
	}

	sort.Strings(files)

	now := time.Now()
	entries := make([]exec.ProcEntry, 0, len(files))
	for _, file := range files {
		entry, err := readProcEntry(file, pg.groupName, pg.staleTime, now)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func freshRefs(entries []exec.ProcEntry) []exec.DAGRunRef {
	seen := make(map[string]exec.DAGRunRef)
	for _, entry := range entries {
		if !entry.Fresh {
			continue
		}
		ref := entry.Meta.DAGRun()
		seen[ref.String()] = ref
	}

	refs := make([]exec.DAGRunRef, 0, len(seen))
	for _, ref := range seen {
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Name == refs[j].Name {
			return refs[i].ID < refs[j].ID
		}
		return refs[i].Name < refs[j].Name
	})
	return refs
}

func latestFreshEntriesByRun(entries []exec.ProcEntry) map[string]exec.ProcEntry {
	latest := make(map[string]exec.ProcEntry)
	for _, entry := range entries {
		if !entry.Fresh {
			continue
		}
		key := entry.Meta.DAGRun().String()
		current, ok := latest[key]
		if !ok || current.LastHeartbeatAt < entry.LastHeartbeatAt {
			latest[key] = entry
		}
	}
	return latest
}

func (pg *ProcGroup) Validate(ctx context.Context) error {
	_, err := pg.ListEntries(ctx)
	if err != nil {
		return fmt.Errorf("validate proc group %s: %w", pg.groupName, err)
	}
	return nil
}
