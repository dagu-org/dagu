// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/dagucloud/dagu/internal/core/exec"
)

var (
	_ ConfigStore                 = (*snapshotConfigStore)(nil)
	_ ModelStore                  = (*snapshotModelStore)(nil)
	_ SkillStore                  = (*snapshotSkillStore)(nil)
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

type snapshotSkillStore struct {
	byID  map[string]*Skill
	items []*Skill
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
		SkillStore:  NewSnapshotSkillStore(snapshot.Skills),
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

func NewSnapshotSkillStore(skills []*Skill) SkillStore {
	if len(skills) == 0 {
		return nil
	}
	items := make([]*Skill, 0, len(skills))
	byID := make(map[string]*Skill, len(skills))
	for _, skill := range skills {
		if skill == nil {
			continue
		}
		items = append(items, skill)
		byID[skill.ID] = skill
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return &snapshotSkillStore{byID: byID, items: items}
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
	for dagName, content := range memory.PerDAG {
		perDAG[dagName] = content
	}
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

func (s *snapshotSkillStore) Create(_ context.Context, _ *Skill) error {
	return ErrSnapshotStoreReadOnly
}

func (s *snapshotSkillStore) GetByID(_ context.Context, id string) (*Skill, error) {
	if err := ValidateSkillID(id); err != nil {
		return nil, err
	}
	skill, ok := s.byID[id]
	if !ok {
		return nil, ErrSkillNotFound
	}
	return skill, nil
}

func (s *snapshotSkillStore) List(_ context.Context) ([]*Skill, error) {
	return append([]*Skill(nil), s.items...), nil
}

func (s *snapshotSkillStore) Search(_ context.Context, opts SearchSkillsOptions) (*exec.PaginatedResult[SkillMetadata], error) {
	queryLower := strings.ToLower(opts.Query)
	matched := make([]SkillMetadata, 0, len(s.items))
	for _, skill := range s.items {
		if opts.AllowedIDs != nil {
			if _, ok := opts.AllowedIDs[skill.ID]; !ok {
				continue
			}
		}
		if len(opts.Tags) > 0 && !snapshotHasAllTags(skill.Tags, opts.Tags) {
			continue
		}
		if queryLower != "" && !snapshotMatchesSkill(skill, queryLower) {
			continue
		}
		matched = append(matched, SkillMetadata{
			ID:            skill.ID,
			Name:          skill.Name,
			Description:   skill.Description,
			Tags:          skill.Tags,
			KnowledgeSize: len(skill.Knowledge),
			Version:       skill.Version,
			Author:        skill.Author,
			Type:          skill.Type,
		})
	}
	return paginateSnapshotResult(matched, opts.Paginator), nil
}

func (s *snapshotSkillStore) Update(_ context.Context, _ *Skill) error {
	return ErrSnapshotStoreReadOnly
}

func (s *snapshotSkillStore) Delete(_ context.Context, _ string) error {
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

func (s *snapshotMemoryStore) SaveGlobalMemory(_ context.Context, _ string) error {
	return ErrSnapshotStoreReadOnly
}

func (s *snapshotMemoryStore) SaveDAGMemory(_ context.Context, _ string, _ string) error {
	return ErrSnapshotStoreReadOnly
}

func (s *snapshotMemoryStore) MemoryDir() string {
	return ""
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

func (s *snapshotMemoryStore) MemoryReadOnly() bool {
	return true
}

func paginateSnapshotResult[T any](items []T, paginator exec.Paginator) *exec.PaginatedResult[T] {
	pg := paginator
	if pg.Limit() == 0 {
		pg = exec.DefaultPaginator()
	}
	total := len(items)
	offset := pg.Offset()
	if offset > total {
		offset = total
	}
	end := offset + pg.Limit()
	if end > total {
		end = total
	}
	result := exec.NewPaginatedResult(items[offset:end], total, pg)
	return &result
}

func snapshotHasAllTags(tags []string, required []string) bool {
	if len(required) == 0 {
		return true
	}
	tagSet := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tagSet[strings.ToLower(tag)] = struct{}{}
	}
	for _, want := range required {
		if _, ok := tagSet[strings.ToLower(want)]; !ok {
			return false
		}
	}
	return true
}

func snapshotMatchesSkill(skill *Skill, query string) bool {
	if strings.Contains(strings.ToLower(skill.ID), query) {
		return true
	}
	if strings.Contains(strings.ToLower(skill.Name), query) {
		return true
	}
	if strings.Contains(strings.ToLower(skill.Description), query) {
		return true
	}
	for _, tag := range skill.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	return false
}

func snapshotMatchesSoul(soul *Soul, query string) bool {
	return strings.Contains(strings.ToLower(soul.ID), query) ||
		strings.Contains(strings.ToLower(soul.Name), query) ||
		strings.Contains(strings.ToLower(soul.Description), query)
}
