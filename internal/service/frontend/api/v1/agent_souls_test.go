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

// soulTestSetup contains test infrastructure for soul API tests.
type soulTestSetup struct {
	api         *apiV1.API
	soulStore   *mockSoulStore
	configStore *mockAgentConfigStore
}

func newSoulTestSetup(t *testing.T) *soulTestSetup {
	t.Helper()

	ss := &mockSoulStore{souls: make(map[string]*agent.Soul)}
	cs := &mockAgentConfigStore{config: agent.DefaultConfig()}

	cfg := &config.Config{}
	a := apiV1.New(
		nil, nil, nil, nil, runtime.Manager{},
		cfg, nil, nil,
		prometheus.NewRegistry(),
		nil,
		apiV1.WithAgentSoulStore(ss),
		apiV1.WithAgentConfigStore(cs),
	)

	return &soulTestSetup{
		api:         a,
		soulStore:   ss,
		configStore: cs,
	}
}

// mockSoulStore is an in-memory implementation of agent.SoulStore.
type mockSoulStore struct {
	souls map[string]*agent.Soul
}

func (m *mockSoulStore) Create(_ context.Context, soul *agent.Soul) error {
	if _, exists := m.souls[soul.ID]; exists {
		return agent.ErrSoulAlreadyExists
	}
	for _, s := range m.souls {
		if s.Name == soul.Name {
			return agent.ErrSoulNameAlreadyExists
		}
	}
	m.souls[soul.ID] = soul
	return nil
}

func (m *mockSoulStore) GetByID(_ context.Context, id string) (*agent.Soul, error) {
	soul, ok := m.souls[id]
	if !ok {
		return nil, agent.ErrSoulNotFound
	}
	return soul, nil
}

func (m *mockSoulStore) List(_ context.Context) ([]*agent.Soul, error) {
	var result []*agent.Soul
	for _, soul := range m.souls {
		result = append(result, soul)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func (m *mockSoulStore) Update(_ context.Context, soul *agent.Soul) error {
	if _, ok := m.souls[soul.ID]; !ok {
		return agent.ErrSoulNotFound
	}
	for _, s := range m.souls {
		if s.Name == soul.Name && s.ID != soul.ID {
			return agent.ErrSoulNameAlreadyExists
		}
	}
	m.souls[soul.ID] = soul
	return nil
}

func (m *mockSoulStore) Delete(_ context.Context, id string) error {
	if _, ok := m.souls[id]; !ok {
		return agent.ErrSoulNotFound
	}
	delete(m.souls, id)
	return nil
}

func (m *mockSoulStore) Search(_ context.Context, opts agent.SearchSoulsOptions) (*exec.PaginatedResult[agent.SoulMetadata], error) {
	var items []agent.SoulMetadata
	for _, s := range m.souls {
		items = append(items, agent.SoulMetadata{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
			Tags:        s.Tags,
			ContentSize: len(s.Content),
			Version:     s.Version,
			Author:      s.Author,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	pg := opts.Paginator
	if pg.Limit() == 0 {
		pg = exec.DefaultPaginator()
	}
	total := len(items)
	offset := min(pg.Offset(), total)
	end := min(offset+pg.Limit(), total)
	result := exec.NewPaginatedResult(items[offset:end], total, pg)
	return &result, nil
}

var _ agent.SoulStore = (*mockSoulStore)(nil)

// Tests for ListAgentSouls

func TestListAgentSouls(t *testing.T) {
	t.Parallel()

	t.Run("returns souls", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)
		setup.soulStore.souls["default"] = &agent.Soul{
			ID: "default", Name: "Default Assistant", Content: "You are a helpful assistant.",
		}
		setup.soulStore.souls["concise-ops"] = &agent.Soul{
			ID: "concise-ops", Name: "Concise Ops", Content: "You are a concise ops assistant.",
		}

		resp, err := setup.api.ListAgentSouls(adminCtx(), apigen.ListAgentSoulsRequestObject{})
		require.NoError(t, err)

		listResp, ok := resp.(apigen.ListAgentSouls200JSONResponse)
		require.True(t, ok)
		require.Len(t, listResp.Souls, 2)

		// Souls are sorted by name
		assert.Equal(t, "Concise Ops", listResp.Souls[0].Name)
		assert.Equal(t, "Default Assistant", listResp.Souls[1].Name)
	})

	t.Run("returns empty list when no souls", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)

		resp, err := setup.api.ListAgentSouls(adminCtx(), apigen.ListAgentSoulsRequestObject{})
		require.NoError(t, err)

		listResp, ok := resp.(apigen.ListAgentSouls200JSONResponse)
		require.True(t, ok)
		assert.Empty(t, listResp.Souls)
	})

	t.Run("returns 403 when store not configured", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		a := apiV1.New(nil, nil, nil, nil, runtime.Manager{}, cfg, nil, nil, prometheus.NewRegistry(), nil)

		_, err := a.ListAgentSouls(adminCtx(), apigen.ListAgentSoulsRequestObject{})
		require.Error(t, err)
	})
}

// Tests for CreateAgentSoul

func TestCreateAgentSoul(t *testing.T) {
	t.Parallel()

	t.Run("valid create returns 201", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)

		resp, err := setup.api.CreateAgentSoul(adminCtx(), apigen.CreateAgentSoulRequestObject{
			Body: &apigen.CreateSoulRequest{
				Name:    "Test Soul",
				Content: "You are a test assistant.",
			},
		})
		require.NoError(t, err)

		createResp, ok := resp.(apigen.CreateAgentSoul201JSONResponse)
		require.True(t, ok)
		assert.Equal(t, "Test Soul", createResp.Name)
		require.NotNil(t, createResp.Content)
		assert.Equal(t, "You are a test assistant.", *createResp.Content)
		assert.NotEmpty(t, createResp.Id)
	})

	t.Run("create with explicit ID", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)

		resp, err := setup.api.CreateAgentSoul(adminCtx(), apigen.CreateAgentSoulRequestObject{
			Body: &apigen.CreateSoulRequest{
				Id:      new("my-custom-soul"),
				Name:    "Custom Soul",
				Content: "Custom identity content.",
			},
		})
		require.NoError(t, err)

		createResp, ok := resp.(apigen.CreateAgentSoul201JSONResponse)
		require.True(t, ok)
		assert.Equal(t, "my-custom-soul", createResp.Id)
	})

	t.Run("create with optional fields", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)

		resp, err := setup.api.CreateAgentSoul(adminCtx(), apigen.CreateAgentSoulRequestObject{
			Body: &apigen.CreateSoulRequest{
				Name:        "Full Soul",
				Content:     "Full identity content.",
				Description: new("A full soul"),
				Version:     new("1.0.0"),
				Author:      new("Test Author"),
				Tags:        &[]string{"tag1", "tag2"},
			},
		})
		require.NoError(t, err)

		createResp, ok := resp.(apigen.CreateAgentSoul201JSONResponse)
		require.True(t, ok)
		assert.Equal(t, "Full Soul", createResp.Name)
		require.NotNil(t, createResp.Description)
		assert.Equal(t, "A full soul", *createResp.Description)
		require.NotNil(t, createResp.Version)
		assert.Equal(t, "1.0.0", *createResp.Version)
		require.NotNil(t, createResp.Author)
		assert.Equal(t, "Test Author", *createResp.Author)
	})

	t.Run("empty name returns error", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)

		_, err := setup.api.CreateAgentSoul(adminCtx(), apigen.CreateAgentSoulRequestObject{
			Body: &apigen.CreateSoulRequest{
				Name:    "",
				Content: "Some content",
			},
		})
		require.Error(t, err)
	})

	t.Run("whitespace-only name returns error", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)

		_, err := setup.api.CreateAgentSoul(adminCtx(), apigen.CreateAgentSoulRequestObject{
			Body: &apigen.CreateSoulRequest{
				Name:    "   ",
				Content: "Some content",
			},
		})
		require.Error(t, err)
	})

	t.Run("empty content returns error", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)

		_, err := setup.api.CreateAgentSoul(adminCtx(), apigen.CreateAgentSoulRequestObject{
			Body: &apigen.CreateSoulRequest{
				Name:    "Bad Soul",
				Content: "",
			},
		})
		require.Error(t, err)
	})

	t.Run("invalid soul ID returns error", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)

		_, err := setup.api.CreateAgentSoul(adminCtx(), apigen.CreateAgentSoulRequestObject{
			Body: &apigen.CreateSoulRequest{
				Id:      new("INVALID ID"),
				Name:    "Test",
				Content: "Test content",
			},
		})
		require.Error(t, err)
	})

	t.Run("duplicate ID returns conflict", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)
		setup.soulStore.souls["existing"] = &agent.Soul{
			ID: "existing", Name: "Existing Soul", Content: "Content",
		}

		_, err := setup.api.CreateAgentSoul(adminCtx(), apigen.CreateAgentSoulRequestObject{
			Body: &apigen.CreateSoulRequest{
				Id:      new("existing"),
				Name:    "Different Name",
				Content: "Test content",
			},
		})
		require.Error(t, err)
	})

	t.Run("duplicate name returns conflict", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)
		setup.soulStore.souls["existing"] = &agent.Soul{
			ID: "existing", Name: "Same Name", Content: "Content",
		}

		_, err := setup.api.CreateAgentSoul(adminCtx(), apigen.CreateAgentSoulRequestObject{
			Body: &apigen.CreateSoulRequest{
				Name:    "Same Name",
				Content: "Test content",
			},
		})
		require.Error(t, err)
	})

	t.Run("nil body returns error", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)

		_, err := setup.api.CreateAgentSoul(adminCtx(), apigen.CreateAgentSoulRequestObject{
			Body: nil,
		})
		require.Error(t, err)
	})
}

// Tests for GetAgentSoul

func TestGetAgentSoul(t *testing.T) {
	t.Parallel()

	t.Run("found returns soul", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)
		setup.soulStore.souls["concise-ops"] = &agent.Soul{
			ID: "concise-ops", Name: "Concise Ops",
			Content: "You are a concise ops assistant.", Description: "Ops soul",
		}

		resp, err := setup.api.GetAgentSoul(adminCtx(), apigen.GetAgentSoulRequestObject{
			SoulId: "concise-ops",
		})
		require.NoError(t, err)

		getResp, ok := resp.(apigen.GetAgentSoul200JSONResponse)
		require.True(t, ok)
		assert.Equal(t, "concise-ops", getResp.Id)
		assert.Equal(t, "Concise Ops", getResp.Name)
		require.NotNil(t, getResp.Content)
		assert.Equal(t, "You are a concise ops assistant.", *getResp.Content)
	})

	t.Run("not found returns error", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)

		_, err := setup.api.GetAgentSoul(adminCtx(), apigen.GetAgentSoulRequestObject{
			SoulId: "nonexistent",
		})
		require.Error(t, err)
	})
}

// Tests for UpdateAgentSoul

func TestUpdateAgentSoul(t *testing.T) {
	t.Parallel()

	t.Run("valid partial update returns 200", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)
		setup.soulStore.souls["my-soul"] = &agent.Soul{
			ID: "my-soul", Name: "Original", Content: "Original content",
		}

		newName := "Updated Name"
		resp, err := setup.api.UpdateAgentSoul(adminCtx(), apigen.UpdateAgentSoulRequestObject{
			SoulId: "my-soul",
			Body: &apigen.UpdateSoulRequest{
				Name: &newName,
			},
		})
		require.NoError(t, err)

		updateResp, ok := resp.(apigen.UpdateAgentSoul200JSONResponse)
		require.True(t, ok)
		assert.Equal(t, "Updated Name", updateResp.Name)
		require.NotNil(t, updateResp.Content)
		assert.Equal(t, "Original content", *updateResp.Content) // unchanged
	})

	t.Run("update content", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)
		setup.soulStore.souls["my-soul"] = &agent.Soul{
			ID: "my-soul", Name: "My Soul", Content: "Old content",
		}

		newContent := "New identity content"
		resp, err := setup.api.UpdateAgentSoul(adminCtx(), apigen.UpdateAgentSoulRequestObject{
			SoulId: "my-soul",
			Body: &apigen.UpdateSoulRequest{
				Content: &newContent,
			},
		})
		require.NoError(t, err)

		updateResp, ok := resp.(apigen.UpdateAgentSoul200JSONResponse)
		require.True(t, ok)
		require.NotNil(t, updateResp.Content)
		assert.Equal(t, "New identity content", *updateResp.Content)
		assert.Equal(t, "My Soul", updateResp.Name) // unchanged
	})

	t.Run("nil fields not applied", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)
		setup.soulStore.souls["my-soul"] = &agent.Soul{
			ID: "my-soul", Name: "Original", Content: "Original content",
			Description: "Original desc",
		}

		resp, err := setup.api.UpdateAgentSoul(adminCtx(), apigen.UpdateAgentSoulRequestObject{
			SoulId: "my-soul",
			Body:   &apigen.UpdateSoulRequest{},
		})
		require.NoError(t, err)

		updateResp, ok := resp.(apigen.UpdateAgentSoul200JSONResponse)
		require.True(t, ok)
		assert.Equal(t, "Original", updateResp.Name)
		require.NotNil(t, updateResp.Content)
		assert.Equal(t, "Original content", *updateResp.Content)
	})

	t.Run("whitespace-only name not applied", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)
		setup.soulStore.souls["my-soul"] = &agent.Soul{
			ID: "my-soul", Name: "Original", Content: "Content",
		}

		emptyName := "  "
		resp, err := setup.api.UpdateAgentSoul(adminCtx(), apigen.UpdateAgentSoulRequestObject{
			SoulId: "my-soul",
			Body: &apigen.UpdateSoulRequest{
				Name: &emptyName,
			},
		})
		require.NoError(t, err)

		updateResp, ok := resp.(apigen.UpdateAgentSoul200JSONResponse)
		require.True(t, ok)
		assert.Equal(t, "Original", updateResp.Name)
	})

	t.Run("name stores trimmed value", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)
		setup.soulStore.souls["my-soul"] = &agent.Soul{
			ID: "my-soul", Name: "Original", Content: "Content",
		}

		paddedName := "  Updated  "
		resp, err := setup.api.UpdateAgentSoul(adminCtx(), apigen.UpdateAgentSoulRequestObject{
			SoulId: "my-soul",
			Body: &apigen.UpdateSoulRequest{
				Name: &paddedName,
			},
		})
		require.NoError(t, err)

		updateResp, ok := resp.(apigen.UpdateAgentSoul200JSONResponse)
		require.True(t, ok)
		assert.Equal(t, "Updated", updateResp.Name)
	})

	t.Run("name conflict returns error", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)
		setup.soulStore.souls["soul-a"] = &agent.Soul{
			ID: "soul-a", Name: "First Soul", Content: "C",
		}
		setup.soulStore.souls["soul-b"] = &agent.Soul{
			ID: "soul-b", Name: "Second Soul", Content: "C",
		}

		conflictName := "First Soul"
		_, err := setup.api.UpdateAgentSoul(adminCtx(), apigen.UpdateAgentSoulRequestObject{
			SoulId: "soul-b",
			Body: &apigen.UpdateSoulRequest{
				Name: &conflictName,
			},
		})
		require.Error(t, err)
	})

	t.Run("not found returns error", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)

		newName := "Updated"
		_, err := setup.api.UpdateAgentSoul(adminCtx(), apigen.UpdateAgentSoulRequestObject{
			SoulId: "nonexistent",
			Body: &apigen.UpdateSoulRequest{
				Name: &newName,
			},
		})
		require.Error(t, err)
	})

	t.Run("nil body returns error", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)

		_, err := setup.api.UpdateAgentSoul(adminCtx(), apigen.UpdateAgentSoulRequestObject{
			SoulId: "my-soul",
			Body:   nil,
		})
		require.Error(t, err)
	})
}

// Tests for DeleteAgentSoul

func TestDeleteAgentSoul(t *testing.T) {
	t.Parallel()

	t.Run("valid delete returns 204", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)
		setup.soulStore.souls["delete-me"] = &agent.Soul{
			ID: "delete-me", Name: "Delete Me", Content: "C",
		}

		resp, err := setup.api.DeleteAgentSoul(adminCtx(), apigen.DeleteAgentSoulRequestObject{
			SoulId: "delete-me",
		})
		require.NoError(t, err)

		_, ok := resp.(apigen.DeleteAgentSoul204Response)
		assert.True(t, ok)

		_, exists := setup.soulStore.souls["delete-me"]
		assert.False(t, exists, "soul should be deleted")
	})

	t.Run("not found returns error", func(t *testing.T) {
		t.Parallel()

		setup := newSoulTestSetup(t)

		_, err := setup.api.DeleteAgentSoul(adminCtx(), apigen.DeleteAgentSoulRequestObject{
			SoulId: "nonexistent",
		})
		require.Error(t, err)
	})
}
