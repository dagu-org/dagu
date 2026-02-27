package api_test

import (
	"context"
	"errors"
	"path"
	"sort"
	"strings"
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

// errForced is a generic error used to trigger internal error paths in the mock.
var errForced = errors.New("forced error")

// mockDocStore is an in-memory implementation of agent.DocStore.
var _ agent.DocStore = (*mockDocStore)(nil)

type mockDocStore struct {
	docs    map[string]*agent.Doc
	failAll bool // when true, all operations return errForced
}

func (m *mockDocStore) Get(_ context.Context, id string) (*agent.Doc, error) {
	if m.failAll {
		return nil, errForced
	}
	if err := agent.ValidateDocID(id); err != nil {
		return nil, agent.ErrInvalidDocID
	}
	doc, ok := m.docs[id]
	if !ok {
		return nil, agent.ErrDocNotFound
	}
	cp := *doc
	return &cp, nil
}

func (m *mockDocStore) Create(_ context.Context, id, content string) error {
	if m.failAll {
		return errForced
	}
	if err := agent.ValidateDocID(id); err != nil {
		return agent.ErrInvalidDocID
	}
	if _, exists := m.docs[id]; exists {
		return agent.ErrDocAlreadyExists
	}
	m.docs[id] = &agent.Doc{
		ID:      id,
		Title:   path.Base(id),
		Content: content,
	}
	return nil
}

func (m *mockDocStore) Update(_ context.Context, id, content string) error {
	if m.failAll {
		return errForced
	}
	if err := agent.ValidateDocID(id); err != nil {
		return agent.ErrInvalidDocID
	}
	doc, ok := m.docs[id]
	if !ok {
		return agent.ErrDocNotFound
	}
	doc.Content = content
	return nil
}

func (m *mockDocStore) Delete(_ context.Context, id string) error {
	if m.failAll {
		return errForced
	}
	if err := agent.ValidateDocID(id); err != nil {
		return agent.ErrInvalidDocID
	}
	if _, ok := m.docs[id]; !ok {
		return agent.ErrDocNotFound
	}
	delete(m.docs, id)
	return nil
}

func (m *mockDocStore) Rename(_ context.Context, oldID, newID string) error {
	if m.failAll {
		return errForced
	}
	if err := agent.ValidateDocID(oldID); err != nil {
		return agent.ErrInvalidDocID
	}
	if err := agent.ValidateDocID(newID); err != nil {
		return agent.ErrInvalidDocID
	}
	doc, ok := m.docs[oldID]
	if !ok {
		return agent.ErrDocNotFound
	}
	if _, exists := m.docs[newID]; exists {
		return agent.ErrDocAlreadyExists
	}
	delete(m.docs, oldID)
	doc.ID = newID
	doc.Title = path.Base(newID)
	m.docs[newID] = doc
	return nil
}

func (m *mockDocStore) Search(_ context.Context, query string) ([]*agent.DocSearchResult, error) {
	if m.failAll {
		return nil, errForced
	}
	var results []*agent.DocSearchResult
	for _, doc := range m.docs {
		if strings.Contains(doc.Content, query) {
			// Build matches from content lines containing the query.
			var matches []*exec.Match
			for i, line := range strings.Split(doc.Content, "\n") {
				if strings.Contains(line, query) {
					matches = append(matches, &exec.Match{
						Line:       line,
						LineNumber: i + 1,
						StartLine:  i + 1,
					})
				}
			}
			results = append(results, &agent.DocSearchResult{
				ID:      doc.ID,
				Title:   doc.Title,
				Matches: matches,
			})
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].ID < results[j].ID })
	return results, nil
}

func (m *mockDocStore) List(_ context.Context, page, perPage int) (*exec.PaginatedResult[*agent.DocTreeNode], error) {
	if m.failAll {
		return nil, errForced
	}
	nodes := make([]*agent.DocTreeNode, 0, len(m.docs))
	for _, doc := range m.docs {
		nodes = append(nodes, &agent.DocTreeNode{
			ID:    doc.ID,
			Name:  path.Base(doc.ID),
			Title: doc.Title,
			Type:  "file",
		})
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })

	pg := exec.NewPaginator(page, perPage)
	start := min(pg.Offset(), len(nodes))
	end := min(start+pg.Limit(), len(nodes))
	result := exec.NewPaginatedResult(nodes[start:end], len(nodes), pg)
	return &result, nil
}

func (m *mockDocStore) ListFlat(_ context.Context, page, perPage int) (*exec.PaginatedResult[agent.DocMetadata], error) {
	if m.failAll {
		return nil, errForced
	}
	items := make([]agent.DocMetadata, 0, len(m.docs))
	for _, doc := range m.docs {
		items = append(items, agent.DocMetadata{
			ID:    doc.ID,
			Title: doc.Title,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })

	pg := exec.NewPaginator(page, perPage)
	start := min(pg.Offset(), len(items))
	end := min(start+pg.Limit(), len(items))
	result := exec.NewPaginatedResult(items[start:end], len(items), pg)
	return &result, nil
}

// docTestSetup contains common test infrastructure for doc API tests.
type docTestSetup struct {
	api   *apiV1.API
	store *mockDocStore
}

func newDocTestSetup(t *testing.T) *docTestSetup {
	t.Helper()
	store := &mockDocStore{docs: make(map[string]*agent.Doc)}
	cfg := &config.Config{}
	cfg.Server.Permissions = map[config.Permission]bool{
		config.PermissionWriteDAGs: true,
	}
	a := apiV1.New(
		nil, nil, nil, nil, runtime.Manager{},
		cfg, nil, nil,
		prometheus.NewRegistry(),
		nil,
		apiV1.WithDocStore(store),
	)
	return &docTestSetup{api: a, store: store}
}

func TestListDocs(t *testing.T) {
	t.Parallel()

	t.Run("flat mode returns items", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)
		setup.store.docs["alpha"] = &agent.Doc{ID: "alpha", Title: "alpha", Content: "content-a"}
		setup.store.docs["beta"] = &agent.Doc{ID: "beta", Title: "beta", Content: "content-b"}

		resp, err := setup.api.ListDocs(adminCtx(), apigen.ListDocsRequestObject{
			Params: apigen.ListDocsParams{
				Flat:    new(true),
				Page:    new(1),
				PerPage: new(10),
			},
		})
		require.NoError(t, err)

		listResp, ok := resp.(apigen.ListDocs200JSONResponse)
		require.True(t, ok)
		require.NotNil(t, listResp.Items)
		assert.Len(t, *listResp.Items, 2)
	})

	t.Run("tree mode returns nodes", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)
		setup.store.docs["doc-a"] = &agent.Doc{ID: "doc-a", Title: "doc-a", Content: "aaa"}
		setup.store.docs["doc-b"] = &agent.Doc{ID: "doc-b", Title: "doc-b", Content: "bbb"}

		resp, err := setup.api.ListDocs(adminCtx(), apigen.ListDocsRequestObject{
			Params: apigen.ListDocsParams{
				Page:    new(1),
				PerPage: new(10),
			},
		})
		require.NoError(t, err)

		listResp, ok := resp.(apigen.ListDocs200JSONResponse)
		require.True(t, ok)
		require.NotNil(t, listResp.Tree)
		assert.Len(t, *listResp.Tree, 2)
	})

	t.Run("no doc store returns error", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		a := apiV1.New(nil, nil, nil, nil, runtime.Manager{}, cfg, nil, nil, prometheus.NewRegistry(), nil)

		_, err := a.ListDocs(adminCtx(), apigen.ListDocsRequestObject{})
		require.Error(t, err)
	})
}

func TestCreateDoc(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)

		resp, err := setup.api.CreateDoc(adminCtx(), apigen.CreateDocRequestObject{
			Body: &apigen.CreateDocJSONRequestBody{
				Id:      "test-doc",
				Content: "hello",
			},
		})
		require.NoError(t, err)

		_, ok := resp.(apigen.CreateDoc201JSONResponse)
		require.True(t, ok)

		// Verify stored
		_, exists := setup.store.docs["test-doc"]
		assert.True(t, exists)
	})

	t.Run("invalid ID", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)

		_, err := setup.api.CreateDoc(adminCtx(), apigen.CreateDocRequestObject{
			Body: &apigen.CreateDocJSONRequestBody{
				Id:      "..bad",
				Content: "x",
			},
		})
		require.Error(t, err)
	})

	t.Run("already exists", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)
		setup.store.docs["existing"] = &agent.Doc{ID: "existing", Title: "existing", Content: "old"}

		_, err := setup.api.CreateDoc(adminCtx(), apigen.CreateDocRequestObject{
			Body: &apigen.CreateDocJSONRequestBody{
				Id:      "existing",
				Content: "new",
			},
		})
		require.Error(t, err)
	})

	t.Run("nil body", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)

		_, err := setup.api.CreateDoc(adminCtx(), apigen.CreateDocRequestObject{
			Body: nil,
		})
		require.Error(t, err)
	})

	t.Run("no doc store", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		a := apiV1.New(nil, nil, nil, nil, runtime.Manager{}, cfg, nil, nil, prometheus.NewRegistry(), nil)

		_, err := a.CreateDoc(adminCtx(), apigen.CreateDocRequestObject{
			Body: &apigen.CreateDocJSONRequestBody{
				Id:      "test",
				Content: "hello",
			},
		})
		require.Error(t, err)
	})
}

func TestGetDoc(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)
		setup.store.docs["my-doc"] = &agent.Doc{ID: "my-doc", Title: "my-doc", Content: "hello"}

		resp, err := setup.api.GetDoc(adminCtx(), apigen.GetDocRequestObject{
			Params: apigen.GetDocParams{Path: "my-doc"},
		})
		require.NoError(t, err)

		getResp, ok := resp.(apigen.GetDoc200JSONResponse)
		require.True(t, ok)
		assert.Equal(t, "my-doc", getResp.Id)
		assert.Equal(t, "hello", getResp.Content)
		assert.Equal(t, "my-doc", getResp.Title)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)

		_, err := setup.api.GetDoc(adminCtx(), apigen.GetDocRequestObject{
			Params: apigen.GetDocParams{Path: "nonexistent"},
		})
		require.Error(t, err)
	})

	t.Run("invalid path", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)

		_, err := setup.api.GetDoc(adminCtx(), apigen.GetDocRequestObject{
			Params: apigen.GetDocParams{Path: "..bad"},
		})
		require.Error(t, err)
	})

	t.Run("no doc store", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		a := apiV1.New(nil, nil, nil, nil, runtime.Manager{}, cfg, nil, nil, prometheus.NewRegistry(), nil)

		_, err := a.GetDoc(adminCtx(), apigen.GetDocRequestObject{
			Params: apigen.GetDocParams{Path: "my-doc"},
		})
		require.Error(t, err)
	})
}

func TestSearchDocs(t *testing.T) {
	t.Parallel()

	t.Run("returns results", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)
		setup.store.docs["doc1"] = &agent.Doc{ID: "doc1", Title: "doc1", Content: "hello world"}
		setup.store.docs["doc2"] = &agent.Doc{ID: "doc2", Title: "doc2", Content: "goodbye world"}
		setup.store.docs["doc3"] = &agent.Doc{ID: "doc3", Title: "doc3", Content: "nothing here"}

		resp, err := setup.api.SearchDocs(adminCtx(), apigen.SearchDocsRequestObject{
			Params: apigen.SearchDocsParams{Q: "world"},
		})
		require.NoError(t, err)

		searchResp, ok := resp.(apigen.SearchDocs200JSONResponse)
		require.True(t, ok)
		assert.Len(t, searchResp.Results, 2)
	})

	t.Run("empty query", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)

		_, err := setup.api.SearchDocs(adminCtx(), apigen.SearchDocsRequestObject{
			Params: apigen.SearchDocsParams{Q: ""},
		})
		require.Error(t, err)
	})

	t.Run("no results", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)
		setup.store.docs["doc1"] = &agent.Doc{ID: "doc1", Title: "doc1", Content: "hello"}

		resp, err := setup.api.SearchDocs(adminCtx(), apigen.SearchDocsRequestObject{
			Params: apigen.SearchDocsParams{Q: "nonexistent-term"},
		})
		require.NoError(t, err)

		searchResp, ok := resp.(apigen.SearchDocs200JSONResponse)
		require.True(t, ok)
		assert.Empty(t, searchResp.Results)
	})

	t.Run("no doc store", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		a := apiV1.New(nil, nil, nil, nil, runtime.Manager{}, cfg, nil, nil, prometheus.NewRegistry(), nil)

		_, err := a.SearchDocs(adminCtx(), apigen.SearchDocsRequestObject{
			Params: apigen.SearchDocsParams{Q: "hello"},
		})
		require.Error(t, err)
	})
}

func TestUpdateDoc(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)
		setup.store.docs["doc1"] = &agent.Doc{ID: "doc1", Title: "doc1", Content: "original"}

		resp, err := setup.api.UpdateDoc(adminCtx(), apigen.UpdateDocRequestObject{
			Params: apigen.UpdateDocParams{Path: "doc1"},
			Body:   &apigen.UpdateDocJSONRequestBody{Content: "updated"},
		})
		require.NoError(t, err)

		_, ok := resp.(apigen.UpdateDoc200JSONResponse)
		require.True(t, ok)

		// Verify store content changed
		assert.Equal(t, "updated", setup.store.docs["doc1"].Content)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)

		_, err := setup.api.UpdateDoc(adminCtx(), apigen.UpdateDocRequestObject{
			Params: apigen.UpdateDocParams{Path: "nonexistent"},
			Body:   &apigen.UpdateDocJSONRequestBody{Content: "updated"},
		})
		require.Error(t, err)
	})

	t.Run("invalid path", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)

		_, err := setup.api.UpdateDoc(adminCtx(), apigen.UpdateDocRequestObject{
			Params: apigen.UpdateDocParams{Path: "..bad"},
			Body:   &apigen.UpdateDocJSONRequestBody{Content: "updated"},
		})
		require.Error(t, err)
	})

	t.Run("nil body", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)

		_, err := setup.api.UpdateDoc(adminCtx(), apigen.UpdateDocRequestObject{
			Params: apigen.UpdateDocParams{Path: "doc1"},
			Body:   nil,
		})
		require.Error(t, err)
	})

	t.Run("no doc store", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		a := apiV1.New(nil, nil, nil, nil, runtime.Manager{}, cfg, nil, nil, prometheus.NewRegistry(), nil)

		_, err := a.UpdateDoc(adminCtx(), apigen.UpdateDocRequestObject{
			Params: apigen.UpdateDocParams{Path: "doc1"},
			Body:   &apigen.UpdateDocJSONRequestBody{Content: "updated"},
		})
		require.Error(t, err)
	})
}

func TestDeleteDoc(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)
		setup.store.docs["doc1"] = &agent.Doc{ID: "doc1", Title: "doc1", Content: "content"}

		resp, err := setup.api.DeleteDoc(adminCtx(), apigen.DeleteDocRequestObject{
			Params: apigen.DeleteDocParams{Path: "doc1"},
		})
		require.NoError(t, err)

		_, ok := resp.(apigen.DeleteDoc204Response)
		require.True(t, ok)

		// Verify removed from store
		_, exists := setup.store.docs["doc1"]
		assert.False(t, exists)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)

		_, err := setup.api.DeleteDoc(adminCtx(), apigen.DeleteDocRequestObject{
			Params: apigen.DeleteDocParams{Path: "nonexistent"},
		})
		require.Error(t, err)
	})

	t.Run("invalid path", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)

		_, err := setup.api.DeleteDoc(adminCtx(), apigen.DeleteDocRequestObject{
			Params: apigen.DeleteDocParams{Path: "..bad"},
		})
		require.Error(t, err)
	})

	t.Run("no doc store", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		a := apiV1.New(nil, nil, nil, nil, runtime.Manager{}, cfg, nil, nil, prometheus.NewRegistry(), nil)

		_, err := a.DeleteDoc(adminCtx(), apigen.DeleteDocRequestObject{
			Params: apigen.DeleteDocParams{Path: "doc1"},
		})
		require.Error(t, err)
	})
}

func TestRenameDoc(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)
		setup.store.docs["old-doc"] = &agent.Doc{ID: "old-doc", Title: "old-doc", Content: "content"}

		resp, err := setup.api.RenameDoc(adminCtx(), apigen.RenameDocRequestObject{
			Params: apigen.RenameDocParams{Path: "old-doc"},
			Body:   &apigen.RenameDocJSONRequestBody{NewPath: "new-doc"},
		})
		require.NoError(t, err)

		_, ok := resp.(apigen.RenameDoc200JSONResponse)
		require.True(t, ok)

		// Verify store has new-doc, not old-doc
		_, oldExists := setup.store.docs["old-doc"]
		assert.False(t, oldExists)
		_, newExists := setup.store.docs["new-doc"]
		assert.True(t, newExists)
	})

	t.Run("source not found", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)

		_, err := setup.api.RenameDoc(adminCtx(), apigen.RenameDocRequestObject{
			Params: apigen.RenameDocParams{Path: "nonexistent"},
			Body:   &apigen.RenameDocJSONRequestBody{NewPath: "new"},
		})
		require.Error(t, err)
	})

	t.Run("target exists", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)
		setup.store.docs["a"] = &agent.Doc{ID: "a", Title: "a", Content: "aaa"}
		setup.store.docs["b"] = &agent.Doc{ID: "b", Title: "b", Content: "bbb"}

		_, err := setup.api.RenameDoc(adminCtx(), apigen.RenameDocRequestObject{
			Params: apigen.RenameDocParams{Path: "a"},
			Body:   &apigen.RenameDocJSONRequestBody{NewPath: "b"},
		})
		require.Error(t, err)
	})

	t.Run("invalid source path", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)

		_, err := setup.api.RenameDoc(adminCtx(), apigen.RenameDocRequestObject{
			Params: apigen.RenameDocParams{Path: "..bad"},
			Body:   &apigen.RenameDocJSONRequestBody{NewPath: "good"},
		})
		require.Error(t, err)
	})

	t.Run("invalid new path", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)
		setup.store.docs["good"] = &agent.Doc{ID: "good", Title: "good", Content: "content"}

		_, err := setup.api.RenameDoc(adminCtx(), apigen.RenameDocRequestObject{
			Params: apigen.RenameDocParams{Path: "good"},
			Body:   &apigen.RenameDocJSONRequestBody{NewPath: "..bad"},
		})
		require.Error(t, err)
	})

	t.Run("nil body", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)

		_, err := setup.api.RenameDoc(adminCtx(), apigen.RenameDocRequestObject{
			Params: apigen.RenameDocParams{Path: "old"},
			Body:   nil,
		})
		require.Error(t, err)
	})

	t.Run("no doc store", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		a := apiV1.New(nil, nil, nil, nil, runtime.Manager{}, cfg, nil, nil, prometheus.NewRegistry(), nil)

		_, err := a.RenameDoc(adminCtx(), apigen.RenameDocRequestObject{
			Params: apigen.RenameDocParams{Path: "old"},
			Body:   &apigen.RenameDocJSONRequestBody{NewPath: "new"},
		})
		require.Error(t, err)
	})
}

func TestSearchDocsWithMatches(t *testing.T) {
	t.Parallel()

	t.Run("returns results with match details", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)
		setup.store.docs["doc1"] = &agent.Doc{
			ID: "doc1", Title: "doc1",
			Content: "line one\nhello world\nline three",
		}

		resp, err := setup.api.SearchDocs(adminCtx(), apigen.SearchDocsRequestObject{
			Params: apigen.SearchDocsParams{Q: "hello"},
		})
		require.NoError(t, err)

		searchResp, ok := resp.(apigen.SearchDocs200JSONResponse)
		require.True(t, ok)
		require.Len(t, searchResp.Results, 1)
		item := searchResp.Results[0]
		assert.Equal(t, "doc1", item.Id)
		require.NotNil(t, item.Matches)
		assert.Len(t, *item.Matches, 1)
		assert.Equal(t, "hello world", (*item.Matches)[0].Line)
		assert.Equal(t, 2, (*item.Matches)[0].LineNumber)
	})
}

func TestListDocsTreeWithChildren(t *testing.T) {
	t.Parallel()

	t.Run("tree nodes with children are rendered", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)
		// Directly put a tree node with children in the store mock.
		// We override List to return a node with children.
		setup.store.docs["parent/child1"] = &agent.Doc{ID: "parent/child1", Title: "child1", Content: "c1"}
		setup.store.docs["parent/child2"] = &agent.Doc{ID: "parent/child2", Title: "child2", Content: "c2"}

		// Replace the store with one that returns a directory structure.
		dirStore := &mockDocStoreWithTree{
			mockDocStore: setup.store,
		}
		cfg := &config.Config{}
		cfg.Server.Permissions = map[config.Permission]bool{
			config.PermissionWriteDAGs: true,
		}
		a := apiV1.New(
			nil, nil, nil, nil, runtime.Manager{},
			cfg, nil, nil,
			prometheus.NewRegistry(),
			nil,
			apiV1.WithDocStore(dirStore),
		)

		resp, err := a.ListDocs(adminCtx(), apigen.ListDocsRequestObject{
			Params: apigen.ListDocsParams{
				Page:    new(1),
				PerPage: new(10),
			},
		})
		require.NoError(t, err)

		listResp, ok := resp.(apigen.ListDocs200JSONResponse)
		require.True(t, ok)
		require.NotNil(t, listResp.Tree)
		require.Len(t, *listResp.Tree, 1)

		parent := (*listResp.Tree)[0]
		assert.Equal(t, "directory", string(parent.Type))
		require.NotNil(t, parent.Children)
		assert.Len(t, *parent.Children, 2)
	})
}

// mockDocStoreWithTree wraps mockDocStore but returns a directory tree from List.
type mockDocStoreWithTree struct {
	*mockDocStore
}

func (m *mockDocStoreWithTree) List(_ context.Context, page, perPage int) (*exec.PaginatedResult[*agent.DocTreeNode], error) {
	nodes := []*agent.DocTreeNode{
		{
			ID:   "parent",
			Name: "parent",
			Type: "directory",
			Children: []*agent.DocTreeNode{
				{ID: "parent/child1", Name: "child1", Title: "child1", Type: "file"},
				{ID: "parent/child2", Name: "child2", Title: "child2", Type: "file"},
			},
		},
	}
	pg := exec.NewPaginator(page, perPage)
	result := exec.NewPaginatedResult(nodes, len(nodes), pg)
	return &result, nil
}

func TestGetDocContentData(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)
		setup.store.docs["doc1"] = &agent.Doc{ID: "doc1", Title: "doc1", Content: "hello"}

		resp, err := setup.api.GetDocContentData(adminCtx(), "doc1")
		require.NoError(t, err)

		docResp, ok := resp.(apigen.DocResponse)
		require.True(t, ok)
		assert.Equal(t, "doc1", docResp.Id)
		assert.Equal(t, "hello", docResp.Content)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		setup := newDocTestSetup(t)

		_, err := setup.api.GetDocContentData(adminCtx(), "nonexistent")
		require.Error(t, err)
	})

	t.Run("no doc store", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		a := apiV1.New(nil, nil, nil, nil, runtime.Manager{}, cfg, nil, nil, prometheus.NewRegistry(), nil)

		_, err := a.GetDocContentData(adminCtx(), "doc1")
		require.Error(t, err)
	})
}

// TestDocStoreInternalErrors covers error paths where the store returns
// unexpected (non-sentinel) errors, triggering the internalError() paths.
func TestDocStoreInternalErrors(t *testing.T) {
	t.Parallel()

	newFailSetup := func(t *testing.T) *docTestSetup {
		t.Helper()
		s := newDocTestSetup(t)
		s.store.failAll = true
		return s
	}

	t.Run("ListDocs flat store error", func(t *testing.T) {
		t.Parallel()
		setup := newFailSetup(t)
		_, err := setup.api.ListDocs(adminCtx(), apigen.ListDocsRequestObject{
			Params: apigen.ListDocsParams{Flat: new(true), Page: new(1), PerPage: new(10)},
		})
		require.Error(t, err)
	})

	t.Run("ListDocs tree store error", func(t *testing.T) {
		t.Parallel()
		setup := newFailSetup(t)
		_, err := setup.api.ListDocs(adminCtx(), apigen.ListDocsRequestObject{
			Params: apigen.ListDocsParams{Page: new(1), PerPage: new(10)},
		})
		require.Error(t, err)
	})

	t.Run("CreateDoc store error", func(t *testing.T) {
		t.Parallel()
		setup := newFailSetup(t)
		_, err := setup.api.CreateDoc(adminCtx(), apigen.CreateDocRequestObject{
			Body: &apigen.CreateDocJSONRequestBody{Id: "test", Content: "hello"},
		})
		require.Error(t, err)
	})

	t.Run("GetDoc store error", func(t *testing.T) {
		t.Parallel()
		setup := newFailSetup(t)
		_, err := setup.api.GetDoc(adminCtx(), apigen.GetDocRequestObject{
			Params: apigen.GetDocParams{Path: "test"},
		})
		require.Error(t, err)
	})

	t.Run("SearchDocs store error", func(t *testing.T) {
		t.Parallel()
		setup := newFailSetup(t)
		_, err := setup.api.SearchDocs(adminCtx(), apigen.SearchDocsRequestObject{
			Params: apigen.SearchDocsParams{Q: "hello"},
		})
		require.Error(t, err)
	})

	t.Run("UpdateDoc store error", func(t *testing.T) {
		t.Parallel()
		setup := newFailSetup(t)
		_, err := setup.api.UpdateDoc(adminCtx(), apigen.UpdateDocRequestObject{
			Params: apigen.UpdateDocParams{Path: "test"},
			Body:   &apigen.UpdateDocJSONRequestBody{Content: "new"},
		})
		require.Error(t, err)
	})

	t.Run("DeleteDoc store error", func(t *testing.T) {
		t.Parallel()
		setup := newFailSetup(t)
		_, err := setup.api.DeleteDoc(adminCtx(), apigen.DeleteDocRequestObject{
			Params: apigen.DeleteDocParams{Path: "test"},
		})
		require.Error(t, err)
	})

	t.Run("RenameDoc store error", func(t *testing.T) {
		t.Parallel()
		setup := newFailSetup(t)
		_, err := setup.api.RenameDoc(adminCtx(), apigen.RenameDocRequestObject{
			Params: apigen.RenameDocParams{Path: "old"},
			Body:   &apigen.RenameDocJSONRequestBody{NewPath: "new"},
		})
		require.Error(t, err)
	})
}

// TestDocWritePermissionDenied covers the requireDAGWrite error path
// when PermissionWriteDAGs is not set.
func TestDocWritePermissionDenied(t *testing.T) {
	t.Parallel()

	newNoWriteSetup := func(t *testing.T) *apiV1.API {
		t.Helper()
		store := &mockDocStore{docs: make(map[string]*agent.Doc)}
		cfg := &config.Config{}
		// Permissions map exists but write is false.
		cfg.Server.Permissions = map[config.Permission]bool{
			config.PermissionWriteDAGs: false,
		}
		return apiV1.New(
			nil, nil, nil, nil, runtime.Manager{},
			cfg, nil, nil,
			prometheus.NewRegistry(),
			nil,
			apiV1.WithDocStore(store),
		)
	}

	t.Run("CreateDoc denied", func(t *testing.T) {
		t.Parallel()
		a := newNoWriteSetup(t)
		_, err := a.CreateDoc(adminCtx(), apigen.CreateDocRequestObject{
			Body: &apigen.CreateDocJSONRequestBody{Id: "test", Content: "hello"},
		})
		require.Error(t, err)
	})

	t.Run("UpdateDoc denied", func(t *testing.T) {
		t.Parallel()
		a := newNoWriteSetup(t)
		_, err := a.UpdateDoc(adminCtx(), apigen.UpdateDocRequestObject{
			Params: apigen.UpdateDocParams{Path: "test"},
			Body:   &apigen.UpdateDocJSONRequestBody{Content: "new"},
		})
		require.Error(t, err)
	})

	t.Run("DeleteDoc denied", func(t *testing.T) {
		t.Parallel()
		a := newNoWriteSetup(t)
		_, err := a.DeleteDoc(adminCtx(), apigen.DeleteDocRequestObject{
			Params: apigen.DeleteDocParams{Path: "test"},
		})
		require.Error(t, err)
	})

	t.Run("RenameDoc denied", func(t *testing.T) {
		t.Parallel()
		a := newNoWriteSetup(t)
		_, err := a.RenameDoc(adminCtx(), apigen.RenameDocRequestObject{
			Params: apigen.RenameDocParams{Path: "old"},
			Body:   &apigen.RenameDocJSONRequestBody{NewPath: "new"},
		})
		require.Error(t, err)
	})
}
