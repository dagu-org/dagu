package filedoc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	return New(dir)
}

func TestCreate(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.Create(ctx, "test-doc", "# Hello\n\nWorld")
	require.NoError(t, err)

	doc, err := store.Get(ctx, "test-doc")
	require.NoError(t, err)
	assert.Equal(t, "test-doc", doc.ID)
	assert.Equal(t, "# Hello\n\nWorld", doc.Content)
}

func TestCreateDuplicate(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, "test-doc", "content"))
	err := store.Create(ctx, "test-doc", "content2")
	assert.ErrorIs(t, err, agent.ErrDocAlreadyExists)
}

func TestCreateNested(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.Create(ctx, "runbooks/deploy/production", "# Deploy Prod")
	require.NoError(t, err)

	doc, err := store.Get(ctx, "runbooks/deploy/production")
	require.NoError(t, err)
	assert.Equal(t, "runbooks/deploy/production", doc.ID)
	assert.Equal(t, "# Deploy Prod", doc.Content)
}

func TestCreateInvalidID(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.Create(ctx, "../escape", "content")
	assert.ErrorIs(t, err, agent.ErrInvalidDocID)

	err = store.Create(ctx, "", "content")
	assert.ErrorIs(t, err, agent.ErrInvalidDocID)
}

func TestGetNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.Get(ctx, "nonexistent")
	assert.ErrorIs(t, err, agent.ErrDocNotFound)
}

func TestUpdate(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, "test-doc", "original"))

	err := store.Update(ctx, "test-doc", "updated content")
	require.NoError(t, err)

	doc, err := store.Get(ctx, "test-doc")
	require.NoError(t, err)
	assert.Equal(t, "updated content", doc.Content)
}

func TestUpdateNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.Update(ctx, "nonexistent", "content")
	assert.ErrorIs(t, err, agent.ErrDocNotFound)
}

func TestDelete(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, "test-doc", "content"))

	err := store.Delete(ctx, "test-doc")
	require.NoError(t, err)

	_, err = store.Get(ctx, "test-doc")
	assert.ErrorIs(t, err, agent.ErrDocNotFound)
}

func TestDeleteCleansEmptyParents(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, "a/b/c", "content"))

	err := store.Delete(ctx, "a/b/c")
	require.NoError(t, err)

	// Parent directories should be cleaned up.
	_, err = os.Stat(filepath.Join(store.baseDir, "a", "b"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(store.baseDir, "a"))
	assert.True(t, os.IsNotExist(err))
}

func TestDeleteNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.Delete(ctx, "nonexistent")
	assert.ErrorIs(t, err, agent.ErrDocNotFound)
}

func TestRename(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, "old-name", "content"))

	err := store.Rename(ctx, "old-name", "new-name")
	require.NoError(t, err)

	_, err = store.Get(ctx, "old-name")
	assert.ErrorIs(t, err, agent.ErrDocNotFound)

	doc, err := store.Get(ctx, "new-name")
	require.NoError(t, err)
	assert.Equal(t, "content", doc.Content)
}

func TestRenameToExisting(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, "doc-a", "content a"))
	require.NoError(t, store.Create(ctx, "doc-b", "content b"))

	err := store.Rename(ctx, "doc-a", "doc-b")
	assert.ErrorIs(t, err, agent.ErrDocAlreadyExists)
}

func TestRenameNested(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, "old/path", "content"))

	err := store.Rename(ctx, "old/path", "new/dir/path")
	require.NoError(t, err)

	doc, err := store.Get(ctx, "new/dir/path")
	require.NoError(t, err)
	assert.Equal(t, "content", doc.Content)

	// Old parent should be cleaned up.
	_, err = os.Stat(filepath.Join(store.baseDir, "old"))
	assert.True(t, os.IsNotExist(err))
}

func TestListFlat(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, "b-doc", "b"))
	require.NoError(t, store.Create(ctx, "a-doc", "a"))
	require.NoError(t, store.Create(ctx, "sub/c-doc", "c"))

	result, err := store.ListFlat(ctx, 1, 50)
	require.NoError(t, err)
	assert.Equal(t, 3, result.TotalCount)

	// Should be sorted alphabetically.
	assert.Equal(t, "a-doc", result.Items[0].ID)
	assert.Equal(t, "b-doc", result.Items[1].ID)
	assert.Equal(t, "sub/c-doc", result.Items[2].ID)
}

func TestListTree(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, "readme", "top level"))
	require.NoError(t, store.Create(ctx, "guides/deploy", "deploy guide"))
	require.NoError(t, store.Create(ctx, "guides/debug", "debug guide"))

	result, err := store.List(ctx, 1, 50)
	require.NoError(t, err)
	assert.Equal(t, 2, result.TotalCount) // 1 dir + 1 file at top level

	// Directories sort before files.
	assert.Equal(t, "directory", result.Items[0].Type)
	assert.Equal(t, "guides", result.Items[0].Name)
	assert.Equal(t, 2, len(result.Items[0].Children))
	assert.Equal(t, "file", result.Items[1].Type)
	assert.Equal(t, "readme.md", result.Items[1].Name)
}

func TestSearch(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, "doc1", "hello world"))
	require.NoError(t, store.Create(ctx, "doc2", "goodbye world"))
	require.NoError(t, store.Create(ctx, "doc3", "nothing here"))

	results, err := store.Search(ctx, "world")
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestSearchNoResults(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, "doc1", "hello"))

	results, err := store.Search(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestFrontmatter(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Write a file with frontmatter directly.
	filePath := filepath.Join(store.baseDir, "with-title.md")
	data := marshalDocFile("My Custom Title", "# Content here")
	require.NoError(t, os.WriteFile(filePath, data, 0600))

	doc, err := store.Get(ctx, "with-title")
	require.NoError(t, err)
	assert.Equal(t, "My Custom Title", doc.Title)
	// Content now includes the full file with frontmatter.
	assert.Equal(t, "---\ntitle: My Custom Title\n---\n# Content here", doc.Content)
}

func TestTitleFromID(t *testing.T) {
	assert.Equal(t, "deploy-guide", titleFromID("runbooks/deploy-guide"))
	assert.Equal(t, "simple", titleFromID("simple"))
}

func TestValidateDocID(t *testing.T) {
	// Valid IDs.
	assert.NoError(t, agent.ValidateDocID("simple"))
	assert.NoError(t, agent.ValidateDocID("with-hyphen"))
	assert.NoError(t, agent.ValidateDocID("with_underscore"))
	assert.NoError(t, agent.ValidateDocID("with space"))
	assert.NoError(t, agent.ValidateDocID("with.dot"))
	assert.NoError(t, agent.ValidateDocID("MixedCase"))
	assert.NoError(t, agent.ValidateDocID("nested/path/doc"))
	assert.NoError(t, agent.ValidateDocID("123"))

	// Invalid IDs.
	assert.ErrorIs(t, agent.ValidateDocID(""), agent.ErrInvalidDocID)
	assert.ErrorIs(t, agent.ValidateDocID("../escape"), agent.ErrInvalidDocID)
	assert.ErrorIs(t, agent.ValidateDocID("/leading-slash"), agent.ErrInvalidDocID)
	assert.ErrorIs(t, agent.ValidateDocID("trailing/"), agent.ErrInvalidDocID)
}

func TestParseDocFileNoFrontmatter(t *testing.T) {
	doc, err := parseDocFile([]byte("# Just markdown\n\nNo frontmatter here"), "test")
	require.NoError(t, err)
	assert.Equal(t, "test", doc.Title) // Title derived from ID.
	assert.Equal(t, "# Just markdown\n\nNo frontmatter here", doc.Content)
}

func TestParseDocFileWithFrontmatter(t *testing.T) {
	input := "---\ntitle: My Doc\n---\n# Content"
	doc, err := parseDocFile([]byte(input), "test")
	require.NoError(t, err)
	assert.Equal(t, "My Doc", doc.Title)
	// Content now includes the full file with frontmatter.
	assert.Equal(t, "---\ntitle: My Doc\n---\n# Content", doc.Content)
}

func TestListFlatSkipsNonConformingFiles(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, "good-doc", "content"))

	// Write a hidden file directly — it should be skipped.
	require.NoError(t, os.WriteFile(filepath.Join(store.baseDir, ".hidden.md"), []byte("hidden"), 0600))

	result, err := store.ListFlat(ctx, 1, 50)
	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalCount)
	assert.Equal(t, "good-doc", result.Items[0].ID)
}

func TestBuildTreeSkipsNonConformingFiles(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, "good-doc", "content"))

	// Write a non-conforming file directly.
	require.NoError(t, os.WriteFile(filepath.Join(store.baseDir, ".hidden.md"), []byte("hidden"), 0600))

	result, err := store.List(ctx, 1, 50)
	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalCount)
	assert.Equal(t, "good-doc", result.Items[0].ID)
}

func TestSearchSkipsNonConformingFiles(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, "good-doc", "findme"))

	// Write a non-conforming file that also contains the search term.
	require.NoError(t, os.WriteFile(filepath.Join(store.baseDir, ".hidden.md"), []byte("findme"), 0600))

	results, err := store.Search(ctx, "findme")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "good-doc", results[0].ID)
}

func TestListFlatPagination(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, "doc-a", "a"))
	require.NoError(t, store.Create(ctx, "doc-b", "b"))
	require.NoError(t, store.Create(ctx, "doc-c", "c"))

	// Page 1 of 2.
	result, err := store.ListFlat(ctx, 1, 2)
	require.NoError(t, err)
	assert.Equal(t, 3, result.TotalCount)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, "doc-a", result.Items[0].ID)
	assert.Equal(t, "doc-b", result.Items[1].ID)

	// Page 2 of 2.
	result, err = store.ListFlat(ctx, 2, 2)
	require.NoError(t, err)
	assert.Equal(t, 3, result.TotalCount)
	assert.Len(t, result.Items, 1)
	assert.Equal(t, "doc-c", result.Items[0].ID)
}

func TestListFlatEmptyPage(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, "doc-a", "a"))

	// Out-of-range page.
	result, err := store.ListFlat(ctx, 10, 50)
	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalCount)
	assert.Empty(t, result.Items)
}

func TestListTreePagination(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, "doc-a", "a"))
	require.NoError(t, store.Create(ctx, "doc-b", "b"))
	require.NoError(t, store.Create(ctx, "doc-c", "c"))

	// Page 1 of 2 top-level nodes.
	result, err := store.List(ctx, 1, 2)
	require.NoError(t, err)
	assert.Equal(t, 3, result.TotalCount)
	assert.Len(t, result.Items, 2)

	// Page 2.
	result, err = store.List(ctx, 2, 2)
	require.NoError(t, err)
	assert.Equal(t, 3, result.TotalCount)
	assert.Len(t, result.Items, 1)
}

func TestDocFilePathTraversal(t *testing.T) {
	store := newTestStore(t)

	_, err := store.docFilePath("../../etc/passwd")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")
}

func TestUpdateWithFrontmatter(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create with frontmatter.
	content := "---\ntitle: Original Title\n---\n# Body"
	require.NoError(t, store.Create(ctx, "fm-doc", content))

	doc, err := store.Get(ctx, "fm-doc")
	require.NoError(t, err)
	assert.Equal(t, "Original Title", doc.Title)
	assert.Equal(t, content, doc.Content)

	// Update with new frontmatter.
	updated := "---\ntitle: New Title\n---\n# Updated Body"
	require.NoError(t, store.Update(ctx, "fm-doc", updated))

	doc, err = store.Get(ctx, "fm-doc")
	require.NoError(t, err)
	assert.Equal(t, "New Title", doc.Title)
	assert.Equal(t, updated, doc.Content)
}

func TestSearchInvalidRegex(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, "doc1", "hello"))

	results, err := store.Search(ctx, "[invalid")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearchEmptyQuery(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, "doc1", "hello"))

	results, err := store.Search(ctx, "")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestBuildTreeExcludesNonMdFiles(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, "real-doc", "content"))

	// Write non-.md files directly.
	require.NoError(t, os.WriteFile(filepath.Join(store.baseDir, "notes.txt"), []byte("text"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(store.baseDir, "config.yaml"), []byte("yaml: true"), 0600))

	result, err := store.List(ctx, 1, 50)
	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalCount)
	assert.Equal(t, "real-doc", result.Items[0].ID)
}

func TestCreateLargeID(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// 256-char ID using nested path to stay within OS filename limits.
	// 5 segments of 50 chars + 5 slashes + 1 char = 256.
	seg := strings.Repeat("a", 50)
	id256 := seg + "/" + seg + "/" + seg + "/" + seg + "/" + seg + "/a"
	require.Len(t, id256, 256)
	err := store.Create(ctx, id256, "content")
	require.NoError(t, err)

	// 257-char ID should fail.
	id257 := id256 + "x"
	err = store.Create(ctx, id257, "content")
	assert.ErrorIs(t, err, agent.ErrInvalidDocID)
}

func TestValidateDocIDBoundary(t *testing.T) {
	// Exactly at the limit.
	assert.NoError(t, agent.ValidateDocID(strings.Repeat("a", 256)))

	// One over the limit.
	assert.ErrorIs(t, agent.ValidateDocID(strings.Repeat("a", 257)), agent.ErrInvalidDocID)
}

func TestTimestamps(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Create(ctx, "ts-doc", "content"))

	doc, err := store.Get(ctx, "ts-doc")
	require.NoError(t, err)
	assert.NotEmpty(t, doc.CreatedAt)
	assert.NotEmpty(t, doc.UpdatedAt)
}
