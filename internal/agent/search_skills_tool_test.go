package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testSkillStore is a SkillStore backed by a fixed list of skills.
type testSkillStore struct {
	skills []*Skill
	err    error
}

func (s *testSkillStore) Create(_ context.Context, _ *Skill) error { return nil }
func (s *testSkillStore) GetByID(_ context.Context, id string) (*Skill, error) {
	for _, sk := range s.skills {
		if sk.ID == id {
			return sk, nil
		}
	}
	return nil, ErrSkillNotFound
}
func (s *testSkillStore) List(_ context.Context) ([]*Skill, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.skills, nil
}

func (s *testSkillStore) Search(_ context.Context, opts SearchSkillsOptions) (*exec.PaginatedResult[SkillMetadata], error) {
	if s.err != nil {
		return nil, s.err
	}

	queryLower := strings.ToLower(opts.Query)
	var matched []SkillMetadata

	for _, sk := range s.skills {
		if opts.AllowedIDs != nil {
			if _, ok := opts.AllowedIDs[sk.ID]; !ok {
				continue
			}
		}
		if len(opts.Tags) > 0 && !testHasAllTags(sk.Tags, opts.Tags) {
			continue
		}
		if queryLower != "" && !testMatchesSkill(sk, queryLower) {
			continue
		}
		matched = append(matched, SkillMetadata{
			ID:          sk.ID,
			Name:        sk.Name,
			Description: sk.Description,
			Tags:        sk.Tags,
		})
	}

	pg := opts.Paginator
	total := len(matched)
	offset := pg.Offset()
	limit := pg.Limit()
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := matched[offset:end]

	result := exec.NewPaginatedResult(page, total, pg)
	return &result, nil
}

func testHasAllTags(skillTags, required []string) bool {
	tagSet := make(map[string]struct{}, len(skillTags))
	for _, t := range skillTags {
		tagSet[strings.ToLower(t)] = struct{}{}
	}
	for _, req := range required {
		if _, ok := tagSet[strings.ToLower(req)]; !ok {
			return false
		}
	}
	return true
}

func testMatchesSkill(skill *Skill, queryLower string) bool {
	if strings.Contains(strings.ToLower(skill.Name), queryLower) {
		return true
	}
	if strings.Contains(strings.ToLower(skill.Description), queryLower) {
		return true
	}
	if strings.Contains(skill.ID, queryLower) {
		return true
	}
	for _, tag := range skill.Tags {
		if strings.Contains(strings.ToLower(tag), queryLower) {
			return true
		}
	}
	return false
}

func (s *testSkillStore) Update(_ context.Context, _ *Skill) error { return nil }
func (s *testSkillStore) Delete(_ context.Context, _ string) error { return nil }

func newTestSkillStore() *testSkillStore {
	return &testSkillStore{
		skills: []*Skill{
			{
				ID:          "sql-optimizer",
				Name:        "SQL Optimizer",
				Description: "Expert in SQL query optimization and database performance",
				Tags:        []string{"sql", "database", "performance"},
				Knowledge:   "SECRET SQL KNOWLEDGE",
			},
			{
				ID:          "docker-deploy",
				Name:        "Docker Deployment",
				Description: "Container orchestration and deployment best practices",
				Tags:        []string{"docker", "deployment", "devops"},
				Knowledge:   "SECRET DOCKER KNOWLEDGE",
			},
			{
				ID:          "go-testing",
				Name:        "Go Testing Patterns",
				Description: "Testing strategies for Go applications",
				Tags:        []string{"go", "testing"},
				Knowledge:   "SECRET GO KNOWLEDGE",
			},
		},
	}
}

func runSearchSkills(t *testing.T, store SkillStore, allowedSkills map[string]struct{}, input any) ToolOut {
	t.Helper()
	tool := NewSearchSkillsTool(store, allowedSkills)
	raw, err := json.Marshal(input)
	require.NoError(t, err)
	return tool.Run(ToolContext{Context: context.Background()}, raw)
}

func TestSearchSkills_EmptyQuery_ReturnsAll(t *testing.T) {
	t.Parallel()
	out := runSearchSkills(t, newTestSkillStore(), nil, map[string]any{})

	assert.Contains(t, out.Content, "Found 3 skill(s)")
	assert.Contains(t, out.Content, "sql-optimizer")
	assert.Contains(t, out.Content, "docker-deploy")
	assert.Contains(t, out.Content, "go-testing")
}

func TestSearchSkills_QueryByName(t *testing.T) {
	t.Parallel()
	out := runSearchSkills(t, newTestSkillStore(), nil, map[string]any{"query": "docker"})

	assert.Contains(t, out.Content, "Found 1 skill(s)")
	assert.Contains(t, out.Content, "docker-deploy")
	assert.NotContains(t, out.Content, "sql-optimizer")
}

func TestSearchSkills_QueryByDescription(t *testing.T) {
	t.Parallel()
	out := runSearchSkills(t, newTestSkillStore(), nil, map[string]any{"query": "orchestration"})

	assert.Contains(t, out.Content, "Found 1 skill(s)")
	assert.Contains(t, out.Content, "docker-deploy")
}

func TestSearchSkills_QueryByTag(t *testing.T) {
	t.Parallel()
	out := runSearchSkills(t, newTestSkillStore(), nil, map[string]any{"query": "performance"})

	assert.Contains(t, out.Content, "Found 1 skill(s)")
	assert.Contains(t, out.Content, "sql-optimizer")
}

func TestSearchSkills_QueryCaseInsensitive(t *testing.T) {
	t.Parallel()
	out := runSearchSkills(t, newTestSkillStore(), nil, map[string]any{"query": "SQL"})

	assert.Contains(t, out.Content, "sql-optimizer")
}

func TestSearchSkills_TagFilter(t *testing.T) {
	t.Parallel()
	out := runSearchSkills(t, newTestSkillStore(), nil, map[string]any{
		"tags": []string{"database"},
	})

	assert.Contains(t, out.Content, "Found 1 skill(s)")
	assert.Contains(t, out.Content, "sql-optimizer")
}

func TestSearchSkills_TagFilterAND(t *testing.T) {
	t.Parallel()
	// Both tags must match.
	out := runSearchSkills(t, newTestSkillStore(), nil, map[string]any{
		"tags": []string{"docker", "deployment"},
	})

	assert.Contains(t, out.Content, "Found 1 skill(s)")
	assert.Contains(t, out.Content, "docker-deploy")

	// No skill has both "sql" and "docker".
	out = runSearchSkills(t, newTestSkillStore(), nil, map[string]any{
		"tags": []string{"sql", "docker"},
	})
	assert.Contains(t, out.Content, "No skills found")
}

func TestSearchSkills_TagFilterCaseInsensitive(t *testing.T) {
	t.Parallel()
	out := runSearchSkills(t, newTestSkillStore(), nil, map[string]any{
		"tags": []string{"SQL"},
	})

	assert.Contains(t, out.Content, "Found 1 skill(s)")
	assert.Contains(t, out.Content, "sql-optimizer")
}

func TestSearchSkills_CombinedQueryAndTags(t *testing.T) {
	t.Parallel()
	// Query matches sql-optimizer, tags also match.
	out := runSearchSkills(t, newTestSkillStore(), nil, map[string]any{
		"query": "optimizer",
		"tags":  []string{"database"},
	})

	assert.Contains(t, out.Content, "Found 1 skill(s)")
	assert.Contains(t, out.Content, "sql-optimizer")

	// Query matches sql-optimizer but tags don't.
	out = runSearchSkills(t, newTestSkillStore(), nil, map[string]any{
		"query": "optimizer",
		"tags":  []string{"docker"},
	})
	assert.Contains(t, out.Content, "No skills found")
}

func TestSearchSkills_AllowedSkillsRestriction(t *testing.T) {
	t.Parallel()
	allowed := map[string]struct{}{
		"go-testing": {},
	}
	out := runSearchSkills(t, newTestSkillStore(), allowed, map[string]any{})

	assert.Contains(t, out.Content, "Found 1 skill(s)")
	assert.Contains(t, out.Content, "go-testing")
	assert.NotContains(t, out.Content, "sql-optimizer")
	assert.NotContains(t, out.Content, "docker-deploy")
}

func TestSearchSkills_AllowedSkillsNilMeansAll(t *testing.T) {
	t.Parallel()
	out := runSearchSkills(t, newTestSkillStore(), nil, map[string]any{})

	assert.Contains(t, out.Content, "Found 3 skill(s)")
}

func TestSearchSkills_NoResults(t *testing.T) {
	t.Parallel()
	out := runSearchSkills(t, newTestSkillStore(), nil, map[string]any{"query": "nonexistent"})

	assert.Contains(t, out.Content, `No skills found matching "nonexistent"`)
}

func TestSearchSkills_NoResultsWithTags(t *testing.T) {
	t.Parallel()
	out := runSearchSkills(t, newTestSkillStore(), nil, map[string]any{
		"tags": []string{"nonexistent"},
	})

	assert.Contains(t, out.Content, "No skills found")
	assert.Contains(t, out.Content, "[nonexistent]")
}

func TestSearchSkills_KnowledgeNotLeaked(t *testing.T) {
	t.Parallel()
	out := runSearchSkills(t, newTestSkillStore(), nil, map[string]any{})

	assert.NotContains(t, out.Content, "SECRET SQL KNOWLEDGE")
	assert.NotContains(t, out.Content, "SECRET DOCKER KNOWLEDGE")
	assert.NotContains(t, out.Content, "SECRET GO KNOWLEDGE")
	assert.NotContains(t, out.Content, "knowledge")
}

func TestSearchSkills_StoreError(t *testing.T) {
	t.Parallel()
	store := &testSkillStore{err: errors.New("store unavailable")}
	out := runSearchSkills(t, store, nil, map[string]any{})

	assert.True(t, out.IsError)
	assert.Contains(t, out.Content, "failed to list skills")
}

func TestSearchSkills_InvalidInput(t *testing.T) {
	t.Parallel()
	tool := NewSearchSkillsTool(newTestSkillStore(), nil)
	out := tool.Run(ToolContext{Context: context.Background()}, json.RawMessage(`{invalid`))

	assert.True(t, out.IsError)
	assert.Contains(t, out.Content, "invalid input")
}

func TestSearchSkills_Pagination(t *testing.T) {
	t.Parallel()
	out := runSearchSkills(t, newTestSkillStore(), nil, map[string]any{
		"per_page": 2,
		"page":     1,
	})

	assert.Contains(t, out.Content, "Found 3 skill(s)")
	assert.Contains(t, out.Content, `"total_count": 3`)
	assert.Contains(t, out.Content, `"current_page": 1`)
	assert.Contains(t, out.Content, `"total_pages": 2`)
	assert.Contains(t, out.Content, `"has_next_page": true`)
}

func TestSearchSkills_PaginationPage2(t *testing.T) {
	t.Parallel()
	out := runSearchSkills(t, newTestSkillStore(), nil, map[string]any{
		"per_page": 2,
		"page":     2,
	})

	assert.Contains(t, out.Content, "Found 3 skill(s)")
	assert.Contains(t, out.Content, `"current_page": 2`)
	assert.Contains(t, out.Content, `"has_next_page": false`)
}

func TestSearchSkills_PaginationDefaultsApplied(t *testing.T) {
	t.Parallel()
	// Default: page=1, per_page=50 — should return all 3 skills in one page.
	out := runSearchSkills(t, newTestSkillStore(), nil, map[string]any{})

	assert.Contains(t, out.Content, `"total_count": 3`)
	assert.Contains(t, out.Content, `"current_page": 1`)
	assert.Contains(t, out.Content, `"total_pages": 1`)
	assert.Contains(t, out.Content, `"has_next_page": false`)
}
