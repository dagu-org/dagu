// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"testing"

	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testSnapshotSkillStore struct {
	skills map[string]*Skill
}

func (s *testSnapshotSkillStore) Create(context.Context, *Skill) error { return nil }
func (s *testSnapshotSkillStore) GetByID(_ context.Context, id string) (*Skill, error) {
	skill, ok := s.skills[id]
	if !ok {
		return nil, ErrSkillNotFound
	}
	return skill, nil
}
func (s *testSnapshotSkillStore) List(context.Context) ([]*Skill, error) { return nil, nil }
func (s *testSnapshotSkillStore) Search(context.Context, SearchSkillsOptions) (*exec.PaginatedResult[SkillMetadata], error) {
	result := exec.NewPaginatedResult([]SkillMetadata{}, 0, exec.DefaultPaginator())
	return &result, nil
}
func (s *testSnapshotSkillStore) Update(context.Context, *Skill) error { return nil }
func (s *testSnapshotSkillStore) Delete(context.Context, string) error { return nil }

type testSnapshotSoulStore struct {
	souls map[string]*Soul
}

func (s *testSnapshotSoulStore) Create(context.Context, *Soul) error { return nil }
func (s *testSnapshotSoulStore) GetByID(_ context.Context, id string) (*Soul, error) {
	soul, ok := s.souls[id]
	if !ok {
		return nil, ErrSoulNotFound
	}
	return soul, nil
}
func (s *testSnapshotSoulStore) List(context.Context) ([]*Soul, error) { return nil, nil }
func (s *testSnapshotSoulStore) Search(context.Context, SearchSoulsOptions) (*exec.PaginatedResult[SoulMetadata], error) {
	result := exec.NewPaginatedResult([]SoulMetadata{}, 0, exec.DefaultPaginator())
	return &result, nil
}
func (s *testSnapshotSoulStore) Update(context.Context, *Soul) error  { return nil }
func (s *testSnapshotSoulStore) Delete(context.Context, string) error { return nil }

func TestMarshalSnapshotRoundTrip(t *testing.T) {
	t.Parallel()

	snapshot := &Snapshot{
		Config: &Config{
			Enabled:        true,
			DefaultModelID: "model-default",
			EnabledSkills:  []string{"skill-global"},
		},
		Models: []*ModelConfig{
			testModelConfig("model-default"),
		},
		Skills: []*Skill{
			{ID: "skill-global", Name: "Global Skill", Knowledge: "knowledge"},
		},
		Souls: []*Soul{
			{ID: "helper", Name: "Helper", Content: "be precise"},
		},
		Memory: &MemorySnapshot{
			Global: "global memory",
			PerDAG: map[string]string{"parent": "parent memory"},
		},
	}

	data, err := MarshalSnapshot(snapshot)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	decoded, err := UnmarshalSnapshot(data)
	require.NoError(t, err)
	require.NotNil(t, decoded)

	assert.Equal(t, SnapshotVersion, decoded.Version)
	require.NotNil(t, decoded.Config)
	assert.Equal(t, "model-default", decoded.Config.DefaultModelID)
	require.Len(t, decoded.Models, 1)
	assert.Equal(t, "model-default", decoded.Models[0].ID)
	require.Len(t, decoded.Skills, 1)
	assert.Equal(t, "skill-global", decoded.Skills[0].ID)
	require.Len(t, decoded.Souls, 1)
	assert.Equal(t, "helper", decoded.Souls[0].ID)
	require.NotNil(t, decoded.Memory)
	assert.Equal(t, "global memory", decoded.Memory.Global)
	assert.Equal(t, "parent memory", decoded.Memory.PerDAG["parent"])
}

func TestUnmarshalSnapshotRejectsUnknownVersion(t *testing.T) {
	t.Parallel()

	data := gzipJSON(t, map[string]any{
		"version": 99,
	})

	snapshot, err := UnmarshalSnapshot(data)
	require.Nil(t, snapshot)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsupportedSnapshotWire))
}

func TestBuildSnapshotForDAG_SkipsNonAgentGraphs(t *testing.T) {
	t.Parallel()

	dag := &core.DAG{
		Name: "plain",
		Steps: []core.Step{
			{Name: "main"},
		},
	}

	data, err := BuildSnapshotForDAG(context.Background(), dag, SnapshotStores{}, SnapshotBuildOptions{})
	require.NoError(t, err)
	require.Nil(t, data)
}

func TestBuildSnapshotForDAG_CapturesLocalSubDAGRequirements(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	configStore := newMockConfigStore(true)
	configStore.config.DefaultModelID = "default-model"
	configStore.config.EnabledSkills = []string{"global-skill"}

	modelStore := newMockModelStore().
		addModel(testModelConfig("default-model")).
		addModel(testModelConfig("child-model"))

	skillStore := &testSnapshotSkillStore{
		skills: map[string]*Skill{
			"global-skill": {ID: "global-skill", Name: "Global Skill", Knowledge: "global"},
			"step-skill":   {ID: "step-skill", Name: "Step Skill", Knowledge: "step"},
		},
	}
	soulStore := &testSnapshotSoulStore{
		souls: map[string]*Soul{
			"helper": {ID: "helper", Name: "Helper", Content: "be precise"},
		},
	}
	memoryStore := newMockMemoryStore()
	memoryStore.global = "global memory"
	memoryStore.dag["parent"] = "parent memory"
	memoryStore.dag["child"] = "child memory"

	child := &core.DAG{
		Name: "child",
		Steps: []core.Step{
			{
				Name: "child-agent",
				Agent: &core.AgentStepConfig{
					Model:  "child-model",
					Skills: []string{"step-skill"},
					Soul:   "helper",
					Memory: &core.AgentMemoryConfig{Enabled: true},
				},
			},
		},
	}
	parent := &core.DAG{
		Name: "parent",
		Steps: []core.Step{
			{
				Name: "root-agent",
				Agent: &core.AgentStepConfig{
					Memory: &core.AgentMemoryConfig{Enabled: true},
				},
			},
			{
				Name:   "call-child",
				SubDAG: &core.SubDAG{Name: "child"},
			},
		},
		LocalDAGs: map[string]*core.DAG{
			"child": child,
		},
	}

	data, err := BuildSnapshotForDAG(ctx, parent, SnapshotStores{
		ConfigStore: configStore,
		ModelStore:  modelStore,
		SkillStore:  skillStore,
		SoulStore:   soulStore,
		MemoryStore: memoryStore,
	}, SnapshotBuildOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, data)

	decoded, err := UnmarshalSnapshot(data)
	require.NoError(t, err)
	require.NotNil(t, decoded)

	assert.Equal(t, []string{"child-model", "default-model"}, modelIDs(decoded.Models))
	assert.Equal(t, []string{"global-skill", "step-skill"}, skillIDs(decoded.Skills))
	assert.Equal(t, []string{"helper"}, soulIDs(decoded.Souls))
	require.NotNil(t, decoded.Memory)
	assert.Equal(t, "global memory", decoded.Memory.Global)
	assert.Equal(t, map[string]string{
		"child":  "child memory",
		"parent": "parent memory",
	}, decoded.Memory.PerDAG)
}

func TestBuildSnapshotForDAG_EnforcesSnapshotSizeLimit(t *testing.T) {
	t.Parallel()

	configStore := newMockConfigStore(true)
	configStore.config.DefaultModelID = "default-model"
	modelStore := newMockModelStore().addModel(testModelConfig("default-model"))

	dag := &core.DAG{
		Name: "agent-dag",
		Steps: []core.Step{
			{
				Name:  "agent-step",
				Agent: &core.AgentStepConfig{},
			},
		},
	}

	data, err := BuildSnapshotForDAG(context.Background(), dag, SnapshotStores{
		ConfigStore: configStore,
		ModelStore:  modelStore,
	}, SnapshotBuildOptions{MaxBytes: 1})
	require.Nil(t, data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent snapshot exceeds max size")
}

func gzipJSON(t *testing.T, v any) []byte {
	t.Helper()

	raw, err := json.Marshal(v)
	require.NoError(t, err)

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err = zw.Write(raw)
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	out, err := io.ReadAll(&buf)
	require.NoError(t, err)
	return out
}

func modelIDs(models []*ModelConfig) []string {
	out := make([]string, 0, len(models))
	for _, model := range models {
		out = append(out, model.ID)
	}
	sort.Strings(out)
	return out
}

func skillIDs(skills []*Skill) []string {
	out := make([]string, 0, len(skills))
	for _, skill := range skills {
		out = append(out, skill.ID)
	}
	sort.Strings(out)
	return out
}

func soulIDs(souls []*Soul) []string {
	out := make([]string, 0, len(souls))
	for _, soul := range souls {
		out = append(out, soul.ID)
	}
	sort.Strings(out)
	return out
}
