package api_test

import (
	"context"
	"sort"
	"testing"

	apigen "github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime"
	apiV1 "github.com/dagu-org/dagu/internal/service/frontend/api/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skillTestSetup contains test infrastructure for skill API tests.
type skillTestSetup struct {
	api         *apiV1.API
	skillStore  *mockSkillStore
	configStore *mockAgentConfigStore
}

func newSkillTestSetup(t *testing.T) *skillTestSetup {
	t.Helper()

	ss := &mockSkillStore{skills: make(map[string]*agent.Skill)}
	cs := &mockAgentConfigStore{config: agent.DefaultConfig()}

	cfg := &config.Config{}
	a := apiV1.New(
		nil, nil, nil, nil, runtime.Manager{},
		cfg, nil, nil,
		prometheus.NewRegistry(),
		nil,
		apiV1.WithAgentSkillStore(ss),
		apiV1.WithAgentConfigStore(cs),
	)

	return &skillTestSetup{
		api:         a,
		skillStore:  ss,
		configStore: cs,
	}
}

// mockSkillStore is an in-memory implementation of agent.SkillStore.
type mockSkillStore struct {
	skills map[string]*agent.Skill
}

func (m *mockSkillStore) Create(_ context.Context, skill *agent.Skill) error {
	if _, exists := m.skills[skill.ID]; exists {
		return agent.ErrSkillAlreadyExists
	}
	for _, s := range m.skills {
		if s.Name == skill.Name {
			return agent.ErrSkillNameAlreadyExists
		}
	}
	m.skills[skill.ID] = skill
	return nil
}

func (m *mockSkillStore) GetByID(_ context.Context, id string) (*agent.Skill, error) {
	skill, ok := m.skills[id]
	if !ok {
		return nil, agent.ErrSkillNotFound
	}
	return skill, nil
}

func (m *mockSkillStore) List(_ context.Context) ([]*agent.Skill, error) {
	var result []*agent.Skill
	for _, skill := range m.skills {
		result = append(result, skill)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func (m *mockSkillStore) Update(_ context.Context, skill *agent.Skill) error {
	if _, ok := m.skills[skill.ID]; !ok {
		return agent.ErrSkillNotFound
	}
	for _, s := range m.skills {
		if s.Name == skill.Name && s.ID != skill.ID {
			return agent.ErrSkillNameAlreadyExists
		}
	}
	m.skills[skill.ID] = skill
	return nil
}

func (m *mockSkillStore) Delete(_ context.Context, id string) error {
	if _, ok := m.skills[id]; !ok {
		return agent.ErrSkillNotFound
	}
	delete(m.skills, id)
	return nil
}

func (m *mockSkillStore) Search(_ context.Context, opts agent.SearchSkillsOptions) (*exec.PaginatedResult[agent.SkillMetadata], error) {
	var items []agent.SkillMetadata
	for _, s := range m.skills {
		items = append(items, agent.SkillMetadata{
			ID:            s.ID,
			Name:          s.Name,
			Description:   s.Description,
			Tags:          s.Tags,
			KnowledgeSize: len(s.Knowledge),
			Version:       s.Version,
			Author:        s.Author,
			Type:          s.Type,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	pg := opts.Paginator
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
	return &result, nil
}

var _ agent.SkillStore = (*mockSkillStore)(nil)

// Tests for ListAgentSkills

func TestListAgentSkills(t *testing.T) {
	t.Parallel()

	t.Run("returns skills with enabled status", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)
		setup.skillStore.skills["k8s"] = &agent.Skill{
			ID: "k8s", Name: "Kubernetes", Type: agent.SkillTypeCustom, Knowledge: "K8s knowledge",
		}
		setup.skillStore.skills["etl"] = &agent.Skill{
			ID: "etl", Name: "ETL Best Practices", Type: agent.SkillTypeCustom, Knowledge: "ETL knowledge",
		}
		setup.configStore.config.EnabledSkills = []string{"k8s"}

		resp, err := setup.api.ListAgentSkills(adminCtx(), apigen.ListAgentSkillsRequestObject{})
		require.NoError(t, err)

		listResp, ok := resp.(apigen.ListAgentSkills200JSONResponse)
		require.True(t, ok)
		require.Len(t, listResp.Skills, 2)

		// Skills are sorted by name
		assert.Equal(t, "ETL Best Practices", listResp.Skills[0].Name)
		assert.False(t, listResp.Skills[0].Enabled)
		assert.Equal(t, "Kubernetes", listResp.Skills[1].Name)
		assert.True(t, listResp.Skills[1].Enabled)
	})

	t.Run("returns empty list when no skills", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)

		resp, err := setup.api.ListAgentSkills(adminCtx(), apigen.ListAgentSkillsRequestObject{})
		require.NoError(t, err)

		listResp, ok := resp.(apigen.ListAgentSkills200JSONResponse)
		require.True(t, ok)
		assert.Empty(t, listResp.Skills)
	})

	t.Run("returns 403 when store not configured", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		a := apiV1.New(nil, nil, nil, nil, runtime.Manager{}, cfg, nil, nil, prometheus.NewRegistry(), nil)

		_, err := a.ListAgentSkills(adminCtx(), apigen.ListAgentSkillsRequestObject{})
		require.Error(t, err)
	})
}

// Tests for CreateAgentSkill

func TestCreateAgentSkill(t *testing.T) {
	t.Parallel()

	t.Run("valid create returns 201", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)

		resp, err := setup.api.CreateAgentSkill(adminCtx(), apigen.CreateAgentSkillRequestObject{
			Body: &apigen.CreateSkillRequest{
				Name:      "Kubernetes",
				Knowledge: "K8s best practices",
			},
		})
		require.NoError(t, err)

		createResp, ok := resp.(apigen.CreateAgentSkill201JSONResponse)
		require.True(t, ok)
		assert.Equal(t, "Kubernetes", createResp.Name)
		require.NotNil(t, createResp.Knowledge)
		assert.Equal(t, "K8s best practices", *createResp.Knowledge)
		assert.NotEmpty(t, createResp.Id)
		assert.Equal(t, apigen.SkillResponseType("custom"), createResp.Type)
		assert.False(t, createResp.Enabled)
	})

	t.Run("create with explicit ID", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)

		resp, err := setup.api.CreateAgentSkill(adminCtx(), apigen.CreateAgentSkillRequestObject{
			Body: &apigen.CreateSkillRequest{
				Id:        new("my-custom-id"),
				Name:      "Custom Skill",
				Knowledge: "Some knowledge",
			},
		})
		require.NoError(t, err)

		createResp, ok := resp.(apigen.CreateAgentSkill201JSONResponse)
		require.True(t, ok)
		assert.Equal(t, "my-custom-id", createResp.Id)
	})

	t.Run("create with optional fields", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)

		resp, err := setup.api.CreateAgentSkill(adminCtx(), apigen.CreateAgentSkillRequestObject{
			Body: &apigen.CreateSkillRequest{
				Name:        "Full Skill",
				Knowledge:   "Knowledge content",
				Description: new("A full skill"),
				Version:     new("1.0.0"),
				Author:      new("Test Author"),
				Tags:        &[]string{"tag1", "tag2"},
			},
		})
		require.NoError(t, err)

		createResp, ok := resp.(apigen.CreateAgentSkill201JSONResponse)
		require.True(t, ok)
		assert.Equal(t, "Full Skill", createResp.Name)
		require.NotNil(t, createResp.Description)
		assert.Equal(t, "A full skill", *createResp.Description)
		require.NotNil(t, createResp.Version)
		assert.Equal(t, "1.0.0", *createResp.Version)
		require.NotNil(t, createResp.Author)
		assert.Equal(t, "Test Author", *createResp.Author)
	})

	t.Run("empty name returns error", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)

		_, err := setup.api.CreateAgentSkill(adminCtx(), apigen.CreateAgentSkillRequestObject{
			Body: &apigen.CreateSkillRequest{
				Name:      "",
				Knowledge: "Some knowledge",
			},
		})
		require.Error(t, err)
	})

	t.Run("whitespace-only name returns error", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)

		_, err := setup.api.CreateAgentSkill(adminCtx(), apigen.CreateAgentSkillRequestObject{
			Body: &apigen.CreateSkillRequest{
				Name:      "   ",
				Knowledge: "Some knowledge",
			},
		})
		require.Error(t, err)
	})

	t.Run("empty knowledge returns error", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)

		_, err := setup.api.CreateAgentSkill(adminCtx(), apigen.CreateAgentSkillRequestObject{
			Body: &apigen.CreateSkillRequest{
				Name:      "Bad Skill",
				Knowledge: "",
			},
		})
		require.Error(t, err)
	})

	t.Run("invalid skill ID returns error", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)

		_, err := setup.api.CreateAgentSkill(adminCtx(), apigen.CreateAgentSkillRequestObject{
			Body: &apigen.CreateSkillRequest{
				Id:        new("INVALID ID"),
				Name:      "Test",
				Knowledge: "Test knowledge",
			},
		})
		require.Error(t, err)
	})

	t.Run("duplicate ID returns conflict", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)
		setup.skillStore.skills["existing"] = &agent.Skill{
			ID: "existing", Name: "Existing Skill", Type: agent.SkillTypeCustom, Knowledge: "K",
		}

		_, err := setup.api.CreateAgentSkill(adminCtx(), apigen.CreateAgentSkillRequestObject{
			Body: &apigen.CreateSkillRequest{
				Id:        new("existing"),
				Name:      "Different Name",
				Knowledge: "Test knowledge",
			},
		})
		require.Error(t, err)
	})

	t.Run("duplicate name returns conflict", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)
		setup.skillStore.skills["existing"] = &agent.Skill{
			ID: "existing", Name: "Same Name", Type: agent.SkillTypeCustom, Knowledge: "K",
		}

		_, err := setup.api.CreateAgentSkill(adminCtx(), apigen.CreateAgentSkillRequestObject{
			Body: &apigen.CreateSkillRequest{
				Name:      "Same Name",
				Knowledge: "Test knowledge",
			},
		})
		require.Error(t, err)
	})

	t.Run("nil body returns error", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)

		_, err := setup.api.CreateAgentSkill(adminCtx(), apigen.CreateAgentSkillRequestObject{
			Body: nil,
		})
		require.Error(t, err)
	})
}

// Tests for GetAgentSkill

func TestGetAgentSkill(t *testing.T) {
	t.Parallel()

	t.Run("found returns skill with enabled status", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)
		setup.skillStore.skills["k8s"] = &agent.Skill{
			ID: "k8s", Name: "Kubernetes", Type: agent.SkillTypeCustom,
			Knowledge: "K8s knowledge", Description: "K8s skill",
		}
		setup.configStore.config.EnabledSkills = []string{"k8s"}

		resp, err := setup.api.GetAgentSkill(adminCtx(), apigen.GetAgentSkillRequestObject{
			SkillId: "k8s",
		})
		require.NoError(t, err)

		getResp, ok := resp.(apigen.GetAgentSkill200JSONResponse)
		require.True(t, ok)
		assert.Equal(t, "k8s", getResp.Id)
		assert.Equal(t, "Kubernetes", getResp.Name)
		assert.True(t, getResp.Enabled)
	})

	t.Run("not found returns error", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)

		_, err := setup.api.GetAgentSkill(adminCtx(), apigen.GetAgentSkillRequestObject{
			SkillId: "nonexistent",
		})
		require.Error(t, err)
	})
}

// Tests for UpdateAgentSkill

func TestUpdateAgentSkill(t *testing.T) {
	t.Parallel()

	t.Run("valid partial update returns 200", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)
		setup.skillStore.skills["my-skill"] = &agent.Skill{
			ID: "my-skill", Name: "Original", Type: agent.SkillTypeCustom,
			Knowledge: "Original knowledge",
		}

		newName := "Updated Name"
		resp, err := setup.api.UpdateAgentSkill(adminCtx(), apigen.UpdateAgentSkillRequestObject{
			SkillId: "my-skill",
			Body: &apigen.UpdateSkillRequest{
				Name: &newName,
			},
		})
		require.NoError(t, err)

		updateResp, ok := resp.(apigen.UpdateAgentSkill200JSONResponse)
		require.True(t, ok)
		assert.Equal(t, "Updated Name", updateResp.Name)
		require.NotNil(t, updateResp.Knowledge)
		assert.Equal(t, "Original knowledge", *updateResp.Knowledge) // unchanged
	})

	t.Run("update knowledge", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)
		setup.skillStore.skills["my-skill"] = &agent.Skill{
			ID: "my-skill", Name: "My Skill", Type: agent.SkillTypeCustom,
			Knowledge: "Old knowledge",
		}

		newKnowledge := "New knowledge content"
		resp, err := setup.api.UpdateAgentSkill(adminCtx(), apigen.UpdateAgentSkillRequestObject{
			SkillId: "my-skill",
			Body: &apigen.UpdateSkillRequest{
				Knowledge: &newKnowledge,
			},
		})
		require.NoError(t, err)

		updateResp, ok := resp.(apigen.UpdateAgentSkill200JSONResponse)
		require.True(t, ok)
		require.NotNil(t, updateResp.Knowledge)
		assert.Equal(t, "New knowledge content", *updateResp.Knowledge)
		assert.Equal(t, "My Skill", updateResp.Name) // unchanged
	})

	t.Run("nil fields not applied", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)
		setup.skillStore.skills["my-skill"] = &agent.Skill{
			ID: "my-skill", Name: "Original", Type: agent.SkillTypeCustom,
			Knowledge: "Original knowledge", Description: "Original desc",
		}

		resp, err := setup.api.UpdateAgentSkill(adminCtx(), apigen.UpdateAgentSkillRequestObject{
			SkillId: "my-skill",
			Body:    &apigen.UpdateSkillRequest{},
		})
		require.NoError(t, err)

		updateResp, ok := resp.(apigen.UpdateAgentSkill200JSONResponse)
		require.True(t, ok)
		assert.Equal(t, "Original", updateResp.Name)
		require.NotNil(t, updateResp.Knowledge)
		assert.Equal(t, "Original knowledge", *updateResp.Knowledge)
	})

	t.Run("empty-string name not applied", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)
		setup.skillStore.skills["my-skill"] = &agent.Skill{
			ID: "my-skill", Name: "Original", Type: agent.SkillTypeCustom,
			Knowledge: "Knowledge",
		}

		emptyName := "  "
		resp, err := setup.api.UpdateAgentSkill(adminCtx(), apigen.UpdateAgentSkillRequestObject{
			SkillId: "my-skill",
			Body: &apigen.UpdateSkillRequest{
				Name: &emptyName,
			},
		})
		require.NoError(t, err)

		updateResp, ok := resp.(apigen.UpdateAgentSkill200JSONResponse)
		require.True(t, ok)
		assert.Equal(t, "Original", updateResp.Name)
	})

	t.Run("name conflict returns error", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)
		setup.skillStore.skills["skill-a"] = &agent.Skill{
			ID: "skill-a", Name: "First Skill", Type: agent.SkillTypeCustom, Knowledge: "K",
		}
		setup.skillStore.skills["skill-b"] = &agent.Skill{
			ID: "skill-b", Name: "Second Skill", Type: agent.SkillTypeCustom, Knowledge: "K",
		}

		conflictName := "First Skill"
		_, err := setup.api.UpdateAgentSkill(adminCtx(), apigen.UpdateAgentSkillRequestObject{
			SkillId: "skill-b",
			Body: &apigen.UpdateSkillRequest{
				Name: &conflictName,
			},
		})
		require.Error(t, err)
	})

	t.Run("not found returns error", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)

		newName := "Updated"
		_, err := setup.api.UpdateAgentSkill(adminCtx(), apigen.UpdateAgentSkillRequestObject{
			SkillId: "nonexistent",
			Body: &apigen.UpdateSkillRequest{
				Name: &newName,
			},
		})
		require.Error(t, err)
	})

	t.Run("nil body returns error", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)

		_, err := setup.api.UpdateAgentSkill(adminCtx(), apigen.UpdateAgentSkillRequestObject{
			SkillId: "my-skill",
			Body:    nil,
		})
		require.Error(t, err)
	})
}

// Tests for DeleteAgentSkill

func TestDeleteAgentSkill(t *testing.T) {
	t.Parallel()

	t.Run("valid delete returns 204", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)
		setup.skillStore.skills["delete-me"] = &agent.Skill{
			ID: "delete-me", Name: "Delete Me", Type: agent.SkillTypeCustom, Knowledge: "K",
		}

		resp, err := setup.api.DeleteAgentSkill(adminCtx(), apigen.DeleteAgentSkillRequestObject{
			SkillId: "delete-me",
		})
		require.NoError(t, err)

		_, ok := resp.(apigen.DeleteAgentSkill204Response)
		assert.True(t, ok)

		_, exists := setup.skillStore.skills["delete-me"]
		assert.False(t, exists, "skill should be deleted")
	})

	t.Run("not found returns error", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)

		_, err := setup.api.DeleteAgentSkill(adminCtx(), apigen.DeleteAgentSkillRequestObject{
			SkillId: "nonexistent",
		})
		require.Error(t, err)
	})

	t.Run("removes from enabled skills on delete", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)
		setup.skillStore.skills["enabled-skill"] = &agent.Skill{
			ID: "enabled-skill", Name: "Enabled Skill", Type: agent.SkillTypeCustom, Knowledge: "K",
		}
		setup.configStore.config.EnabledSkills = []string{"enabled-skill", "other-skill"}

		_, err := setup.api.DeleteAgentSkill(adminCtx(), apigen.DeleteAgentSkillRequestObject{
			SkillId: "enabled-skill",
		})
		require.NoError(t, err)

		// enabled-skill should be removed from enabled list
		assert.Equal(t, []string{"other-skill"}, setup.configStore.config.EnabledSkills)
	})
}

// Tests for SetEnabledSkills

func TestSetEnabledSkills(t *testing.T) {
	t.Parallel()

	t.Run("valid set returns 200", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)
		setup.skillStore.skills["skill-a"] = &agent.Skill{
			ID: "skill-a", Name: "Skill A", Type: agent.SkillTypeCustom, Knowledge: "K",
		}
		setup.skillStore.skills["skill-b"] = &agent.Skill{
			ID: "skill-b", Name: "Skill B", Type: agent.SkillTypeCustom, Knowledge: "K",
		}

		resp, err := setup.api.SetEnabledSkills(adminCtx(), apigen.SetEnabledSkillsRequestObject{
			Body: &apigen.SetEnabledSkillsRequest{
				SkillIds: []string{"skill-a", "skill-b"},
			},
		})
		require.NoError(t, err)

		setResp, ok := resp.(apigen.SetEnabledSkills200JSONResponse)
		require.True(t, ok)
		assert.ElementsMatch(t, []string{"skill-a", "skill-b"}, setResp.EnabledSkills)
	})

	t.Run("empty list disables all", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)
		setup.configStore.config.EnabledSkills = []string{"skill-a"}

		resp, err := setup.api.SetEnabledSkills(adminCtx(), apigen.SetEnabledSkillsRequestObject{
			Body: &apigen.SetEnabledSkillsRequest{
				SkillIds: []string{},
			},
		})
		require.NoError(t, err)

		setResp, ok := resp.(apigen.SetEnabledSkills200JSONResponse)
		require.True(t, ok)
		assert.Empty(t, setResp.EnabledSkills)
	})

	t.Run("nonexistent skill returns error", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)

		_, err := setup.api.SetEnabledSkills(adminCtx(), apigen.SetEnabledSkillsRequestObject{
			Body: &apigen.SetEnabledSkillsRequest{
				SkillIds: []string{"nonexistent"},
			},
		})
		require.Error(t, err)
	})

	t.Run("nil body returns error", func(t *testing.T) {
		t.Parallel()

		setup := newSkillTestSetup(t)

		_, err := setup.api.SetEnabledSkills(adminCtx(), apigen.SetEnabledSkillsRequestObject{
			Body: nil,
		})
		require.Error(t, err)
	})
}
