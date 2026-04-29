// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/dagucloud/dagu/internal/core/exec"
)

var (
	_ ConfigStore                 = (*snapshotConfigStore)(nil)
	_ ModelStore                  = (*snapshotModelStore)(nil)
	_ SoulStore                   = (*snapshotSoulStore)(nil)
	_ MemoryStore                 = (*snapshotMemoryStore)(nil)
	_ SnapshotReadOnlyMemoryStore = (*snapshotMemoryStore)(nil)
)

type snapshotConfigStore struct {
	cfg *Config
}

type snapshotModelStore struct {
	byID  map[string]*ModelConfig
	items []*ModelConfig
}

type snapshotSoulStore struct {
	byID  map[string]*Soul
	items []*Soul
}

type snapshotMemoryStore struct {
	global string
	perDAG map[string]string
}

// NewSnapshotStores hydrates read-only in-memory stores from a worker snapshot.
func NewSnapshotStores(snapshot *Snapshot) SnapshotStores {
	if snapshot == nil {
		return SnapshotStores{}
	}

	stores := SnapshotStores{
		ConfigStore: NewSnapshotConfigStore(snapshot.Config),
		ModelStore:  NewSnapshotModelStore(snapshot.Models),
		SoulStore:   NewSnapshotSoulStore(snapshot.Souls),
	}
	if snapshot.Memory != nil {
		stores.MemoryStore = NewSnapshotMemoryStore(snapshot.Memory)
	}
	return stores
}

func NewSnapshotConfigStore(cfg *Config) ConfigStore {
	if cfg == nil {
		return nil
	}
	return &snapshotConfigStore{cfg: cfg}
}

func NewSnapshotModelStore(models []*ModelConfig) ModelStore {
	if len(models) == 0 {
		return nil
	}
	items := make([]*ModelConfig, 0, len(models))
	byID := make(map[string]*ModelConfig, len(models))
	for _, model := range models {
		if model == nil {
			continue
		}
		items = append(items, model)
		byID[model.ID] = model
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return &snapshotModelStore{byID: byID, items: items}
}

func NewSnapshotSoulStore(souls []*Soul) SoulStore {
	if len(souls) == 0 {
		return nil
	}
	items := make([]*Soul, 0, len(souls))
	byID := make(map[string]*Soul, len(souls))
	for _, soul := range souls {
		if soul == nil {
			continue
		}
		items = append(items, soul)
		byID[soul.ID] = soul
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return &snapshotSoulStore{byID: byID, items: items}
}

func NewSnapshotMemoryStore(memory *MemorySnapshot) MemoryStore {
	if memory == nil {
		return nil
	}
	perDAG := make(map[string]string, len(memory.PerDAG))
	maps.Copy(perDAG, memory.PerDAG)
	return &snapshotMemoryStore{
		global: memory.Global,
		perDAG: perDAG,
	}
}

func (s *snapshotConfigStore) Load(_ context.Context) (*Config, error) {
	return s.cfg, nil
}

func (s *snapshotConfigStore) Save(_ context.Context, _ *Config) error {
	return ErrSnapshotStoreReadOnly
}

func (s *snapshotConfigStore) IsEnabled(_ context.Context) bool {
	return s.cfg != nil && s.cfg.Enabled
}

func (s *snapshotModelStore) Create(_ context.Context, _ *ModelConfig) error {
	return ErrSnapshotStoreReadOnly
}

func (s *snapshotModelStore) GetByID(_ context.Context, id string) (*ModelConfig, error) {
	if err := ValidateModelID(id); err != nil {
		return nil, err
	}
	model, ok := s.byID[id]
	if !ok {
		return nil, ErrModelNotFound
	}
	return model, nil
}

func (s *snapshotModelStore) List(_ context.Context) ([]*ModelConfig, error) {
	return append([]*ModelConfig(nil), s.items...), nil
}

func (s *snapshotModelStore) Update(_ context.Context, _ *ModelConfig) error {
	return ErrSnapshotStoreReadOnly
}

func (s *snapshotModelStore) Delete(_ context.Context, _ string) error {
	return ErrSnapshotStoreReadOnly
}

func (s *snapshotSoulStore) Create(_ context.Context, _ *Soul) error {
	return ErrSnapshotStoreReadOnly
}

func (s *snapshotSoulStore) GetByID(_ context.Context, id string) (*Soul, error) {
	if err := ValidateSoulID(id); err != nil {
		return nil, err
	}
	soul, ok := s.byID[id]
	if !ok {
		return nil, ErrSoulNotFound
	}
	return soul, nil
}

func (s *snapshotSoulStore) List(_ context.Context) ([]*Soul, error) {
	return append([]*Soul(nil), s.items...), nil
}

func (s *snapshotSoulStore) Search(_ context.Context, opts SearchSoulsOptions) (*exec.PaginatedResult[SoulMetadata], error) {
	queryLower := strings.ToLower(opts.Query)
	matched := make([]SoulMetadata, 0, len(s.items))
	for _, soul := range s.items {
		if queryLower != "" && !snapshotMatchesSoul(soul, queryLower) {
			continue
		}
		matched = append(matched, SoulMetadata{
			ID:          soul.ID,
			Name:        soul.Name,
			Description: soul.Description,
			ContentSize: len(soul.Content),
		})
	}
	return paginateSnapshotResult(matched, opts.Paginator), nil
}

func (s *snapshotSoulStore) Update(_ context.Context, _ *Soul) error {
	return ErrSnapshotStoreReadOnly
}

func (s *snapshotSoulStore) Delete(_ context.Context, _ string) error {
	return ErrSnapshotStoreReadOnly
}

func (s *snapshotMemoryStore) LoadGlobalMemory(_ context.Context) (string, error) {
	return s.global, nil
}

func (s *snapshotMemoryStore) LoadDAGMemory(_ context.Context, dagName string) (string, error) {
	if strings.TrimSpace(dagName) == "" {
		return "", fmt.Errorf("snapshot memory store: dagName cannot be empty")
	}
	return s.perDAG[dagName], nil
}

func (s *snapshotMemoryStore) LoadAutopilotMemory(_ context.Context, autopilotName string) (string, error) {
	if strings.TrimSpace(autopilotName) == "" {
		return "", fmt.Errorf("snapshot memory store: autopilotName cannot be empty")
	}
	return "", nil
}

func (s *snapshotMemoryStore) SaveGlobalMemory(_ context.Context, _ string) error {
	return ErrSnapshotStoreReadOnly
}

func (s *snapshotMemoryStore) SaveDAGMemory(_ context.Context, _ string, _ string) error {
	return ErrSnapshotStoreReadOnly
}

func (s *snapshotMemoryStore) SaveAutopilotMemory(_ context.Context, _ string, _ string) error {
	return ErrSnapshotStoreReadOnly
}

func (s *snapshotMemoryStore) MemoryDir() string {
	return ""
}

func (s *snapshotMemoryStore) AutopilotMemoryPath(autopilotName string) (string, error) {
	if strings.TrimSpace(autopilotName) == "" {
		return "", fmt.Errorf("snapshot memory store: autopilotName cannot be empty")
	}
	return "", nil
}

func (s *snapshotMemoryStore) ListDAGMemories(_ context.Context) ([]string, error) {
	names := make([]string, 0, len(s.perDAG))
	for dagName := range s.perDAG {
		names = append(names, dagName)
	}
	sort.Strings(names)
	return names, nil
}

func (s *snapshotMemoryStore) DeleteGlobalMemory(_ context.Context) error {
	return ErrSnapshotStoreReadOnly
}

func (s *snapshotMemoryStore) DeleteDAGMemory(_ context.Context, _ string) error {
	return ErrSnapshotStoreReadOnly
}

func (s *snapshotMemoryStore) DeleteAutopilotMemory(_ context.Context, _ string) error {
	return ErrSnapshotStoreReadOnly
}

func (s *snapshotMemoryStore) MemoryReadOnly() bool {
	return true
}

func paginateSnapshotResult[T any](items []T, paginator exec.Paginator) *exec.PaginatedResult[T] {
	pg := paginator
	if pg.Limit() == 0 {
		pg = exec.DefaultPaginator()
	}
	total := len(items)
	offset := min(pg.Offset(), total)
	end := min(offset+pg.Limit(), total)
	result := exec.NewPaginatedResult(items[offset:end], total, pg)
	return &result
}

func snapshotMatchesSoul(soul *Soul, query string) bool {
	return strings.Contains(strings.ToLower(soul.ID), query) ||
		strings.Contains(strings.ToLower(soul.Name), query) ||
		strings.Contains(strings.ToLower(soul.Description), query)
}
