package localdag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-suspend-checker")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	dr := New(tmpDir)
	ctx := context.Background()

	require.False(t, dr.IsSuspended(ctx, "test"))

	err := dr.ToggleSuspend(ctx, "test", true)
	require.NoError(t, err)

	require.True(t, dr.IsSuspended(ctx, "test"))
}

func TestListDAGsInSubdirectories(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-list-subdirs")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	store := New(tmpDir)
	ctx := context.Background()

	// Create DAG files in different directory levels
	// Root level DAG
	rootDAG := `name: root-dag
steps:
  - name: step1
    command: echo "root"`
	err := os.WriteFile(filepath.Join(tmpDir, "root-dag.yaml"), []byte(rootDAG), 0644)
	require.NoError(t, err)

	// Create subdirectory and DAG
	subDir := filepath.Join(tmpDir, "subdir")
	err = os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	subDAG := `name: sub-dag
steps:
  - name: step1
    command: echo "sub"`
	err = os.WriteFile(filepath.Join(subDir, "sub-dag.yaml"), []byte(subDAG), 0644)
	require.NoError(t, err)

	// Create nested subdirectory and DAG
	nestedDir := filepath.Join(tmpDir, "subdir", "nested")
	err = os.MkdirAll(nestedDir, 0755)
	require.NoError(t, err)

	nestedDAG := `name: nested-dag
steps:
  - name: step1
    command: echo "nested"`
	err = os.WriteFile(filepath.Join(nestedDir, "nested-dag.yaml"), []byte(nestedDAG), 0644)
	require.NoError(t, err)

	// List all DAGs
	opts := models.ListDAGsOptions{}
	result, errList, err := store.List(ctx, opts)
	require.NoError(t, err)
	require.Empty(t, errList)

	// Should find only 1 DAG (the root-level one)
	require.Len(t, result.Items, 1, "Should only find DAGs in root directory, not subdirectories")

	// Verify only the root DAG is found
	require.Equal(t, "root-dag", result.Items[0].Name, "Should only find root-dag")
}

func TestGetMetadata(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-get-metadata")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	store := New(tmpDir)
	ctx := context.Background()

	// Test successful metadata retrieval
	dagContent := `name: test-dag
tags: ["tag1", "tag2"]
steps:
  - name: step1
    command: echo "hello"`
	err := os.WriteFile(filepath.Join(tmpDir, "test-dag.yaml"), []byte(dagContent), 0644)
	require.NoError(t, err)

	dag, err := store.GetMetadata(ctx, "test-dag")
	require.NoError(t, err)
	require.NotNil(t, dag)
	assert.Equal(t, "test-dag", dag.Name)
	assert.Equal(t, []string{"tag1", "tag2"}, dag.Tags)

	// Test DAG not found
	_, err = store.GetMetadata(ctx, "non-existent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to locate DAG non-existent")
}

func TestGetDetails(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-get-details")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	store := New(tmpDir)
	ctx := context.Background()

	// Test successful details retrieval
	dagContent := `name: detailed-dag
schedule: "0 1 * * *"
steps:
  - name: step1
    command: echo "detailed"`
	err := os.WriteFile(filepath.Join(tmpDir, "detailed-dag.yaml"), []byte(dagContent), 0644)
	require.NoError(t, err)

	dag, err := store.GetDetails(ctx, "detailed-dag")
	require.NoError(t, err)
	require.NotNil(t, dag)
	assert.Equal(t, "detailed-dag", dag.Name)
	require.Len(t, dag.Schedule, 1)
	assert.Equal(t, "0 1 * * *", dag.Schedule[0].Expression)

	// Test DAG not found
	_, err = store.GetDetails(ctx, "non-existent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to locate DAG non-existent")
}

func TestGetSpec(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-get-spec")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	store := New(tmpDir)
	ctx := context.Background()

	// Test successful spec retrieval
	dagContent := `name: spec-dag
steps:
  - name: step1
    command: echo "spec"`
	err := os.WriteFile(filepath.Join(tmpDir, "spec-dag.yaml"), []byte(dagContent), 0644)
	require.NoError(t, err)

	spec, err := store.GetSpec(ctx, "spec-dag")
	require.NoError(t, err)
	assert.Equal(t, dagContent, spec)

	// Test DAG not found
	_, err = store.GetSpec(ctx, "non-existent")
	require.Error(t, err)
	assert.Equal(t, models.ErrDAGNotFound, err)
}

func TestCreate(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-create")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	store := New(tmpDir)
	ctx := context.Background()

	// Test successful creation
	dagContent := `name: new-dag
steps:
  - name: step1
    command: echo "new"`
	err := store.Create(ctx, "new-dag", []byte(dagContent))
	require.NoError(t, err)

	// Verify file was created
	filePath := filepath.Join(tmpDir, "new-dag.yaml")
	assert.True(t, fileExists(filePath))

	// Verify content
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, dagContent, string(content))

	// Test creating duplicate DAG
	err = store.Create(ctx, "new-dag", []byte(dagContent))
	require.Error(t, err)
	assert.Equal(t, models.ErrDAGAlreadyExists, err)
}

func TestUpdateSpec(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-update-spec")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	store := New(tmpDir)
	ctx := context.Background()

	// Create initial DAG
	initialContent := `name: update-dag
steps:
  - name: step1
    command: echo "initial"`
	err := store.Create(ctx, "update-dag", []byte(initialContent))
	require.NoError(t, err)

	// Test successful update
	updatedContent := `name: update-dag
steps:
  - name: step1
    command: echo "updated"`
	err = store.UpdateSpec(ctx, "update-dag", []byte(updatedContent))
	require.NoError(t, err)

	// Verify content was updated
	spec, err := store.GetSpec(ctx, "update-dag")
	require.NoError(t, err)
	assert.Equal(t, updatedContent, spec)

	// Test updating non-existent DAG
	err = store.UpdateSpec(ctx, "non-existent", []byte(updatedContent))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to locate DAG non-existent")

	// Test updating with invalid spec
	invalidContent := `invalid: yaml: content`
	err = store.UpdateSpec(ctx, "update-dag", []byte(invalidContent))
	require.Error(t, err)
}

func TestDelete(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-delete")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	store := New(tmpDir)
	ctx := context.Background()

	// Create DAG to delete
	dagContent := `name: delete-dag
steps:
  - name: step1
    command: echo "delete"`
	err := store.Create(ctx, "delete-dag", []byte(dagContent))
	require.NoError(t, err)

	// Verify file exists
	filePath := filepath.Join(tmpDir, "delete-dag.yaml")
	assert.True(t, fileExists(filePath))

	// Test successful deletion
	err = store.Delete(ctx, "delete-dag")
	require.NoError(t, err)

	// Verify file was deleted
	assert.False(t, fileExists(filePath))

	// Test deleting non-existent DAG (should not error)
	err = store.Delete(ctx, "non-existent")
	require.NoError(t, err)
}

func TestRename(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-rename")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	store := New(tmpDir)
	ctx := context.Background()

	// Create DAG to rename
	dagContent := `name: old-name
steps:
  - name: step1
    command: echo "rename"`
	err := store.Create(ctx, "old-name", []byte(dagContent))
	require.NoError(t, err)

	// Test successful rename
	err = store.Rename(ctx, "old-name", "new-name")
	require.NoError(t, err)

	// Verify old file doesn't exist
	oldPath := filepath.Join(tmpDir, "old-name.yaml")
	assert.False(t, fileExists(oldPath))

	// Verify new file exists
	newPath := filepath.Join(tmpDir, "new-name.yaml")
	assert.True(t, fileExists(newPath))

	// Verify content is preserved
	spec, err := store.GetSpec(ctx, "new-name")
	require.NoError(t, err)
	assert.Equal(t, dagContent, spec)

	// Test renaming non-existent DAG
	err = store.Rename(ctx, "non-existent", "new-name")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to locate DAG non-existent")

	// Test renaming to existing name
	err = store.Create(ctx, "another-dag", []byte(dagContent))
	require.NoError(t, err)
	err = store.Rename(ctx, "new-name", "another-dag")
	require.Error(t, err)
	assert.Equal(t, models.ErrDAGAlreadyExists, err)
}

func TestGrep(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-grep")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	store := New(tmpDir)
	ctx := context.Background()

	// Create DAGs with different content
	dag1Content := `name: search-dag-1
steps:
  - name: step1
    command: echo "search pattern here"`
	err := store.Create(ctx, "search-dag-1", []byte(dag1Content))
	require.NoError(t, err)

	dag2Content := `name: search-dag-2
steps:
  - name: step1
    command: echo "no match here"`
	err = store.Create(ctx, "search-dag-2", []byte(dag2Content))
	require.NoError(t, err)

	dag3Content := `name: search-dag-3
description: "This contains the search pattern"
steps:
  - name: step1
    command: echo "hello"`
	err = store.Create(ctx, "search-dag-3", []byte(dag3Content))
	require.NoError(t, err)

	// Test successful grep
	results, errList, err := store.Grep(ctx, "search pattern")
	require.NoError(t, err)
	require.Empty(t, errList)
	require.Len(t, results, 2)

	// Verify correct DAGs are found
	foundNames := make(map[string]bool)
	for _, result := range results {
		foundNames[result.Name] = true
		assert.NotEmpty(t, result.Matches)
	}
	assert.True(t, foundNames["search-dag-1"])
	assert.True(t, foundNames["search-dag-3"])
	assert.False(t, foundNames["search-dag-2"])

	// Test grep with no pattern (should return empty)
	results, errList, err = store.Grep(ctx, "")
	require.NoError(t, err)
	require.Empty(t, errList)
	require.Empty(t, results)

	// Test grep with non-matching pattern
	results, errList, err = store.Grep(ctx, "nonexistentpattern")
	require.NoError(t, err)
	require.Empty(t, errList)
	require.Empty(t, results)
}

func TestTagList(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-tag-list")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	store := New(tmpDir)
	ctx := context.Background()

	// Create DAGs with different tags
	dag1Content := `name: tag-dag-1
tags: ["web", "daily"]
steps:
  - name: step1
    command: echo "tag1"`
	err := store.Create(ctx, "tag-dag-1", []byte(dag1Content))
	require.NoError(t, err)

	dag2Content := `name: tag-dag-2
tags: ["batch", "daily", "weekly"]
steps:
  - name: step1
    command: echo "tag2"`
	err = store.Create(ctx, "tag-dag-2", []byte(dag2Content))
	require.NoError(t, err)

	dag3Content := `name: tag-dag-3
steps:
  - name: step1
    command: echo "no tags"`
	err = store.Create(ctx, "tag-dag-3", []byte(dag3Content))
	require.NoError(t, err)

	// Test tag list
	tags, errList, err := store.TagList(ctx)
	require.NoError(t, err)
	require.Empty(t, errList)
	require.Len(t, tags, 4)

	// Verify all unique tags are present
	tagSet := make(map[string]bool)
	for _, tag := range tags {
		tagSet[tag] = true
	}
	assert.True(t, tagSet["web"])
	assert.True(t, tagSet["daily"])
	assert.True(t, tagSet["batch"])
	assert.True(t, tagSet["weekly"])
}

func TestLoadSpec(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-load-spec")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	store := New(tmpDir)
	ctx := context.Background()

	// Test valid spec
	validSpec := `name: load-spec-dag
steps:
  - name: step1
    command: echo "load spec"`
	dag, err := store.LoadSpec(ctx, []byte(validSpec))
	require.NoError(t, err)
	require.NotNil(t, dag)
	assert.Equal(t, "load-spec-dag", dag.Name)

	// Test invalid spec
	invalidSpec := `invalid: yaml: content: [unclosed`
	_, err = store.LoadSpec(ctx, []byte(invalidSpec))
	require.Error(t, err)
}

func TestListWithPagination(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-list-pagination")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	store := New(tmpDir)
	ctx := context.Background()

	// Create multiple DAGs
	for i := 1; i <= 5; i++ {
		dagContent := fmt.Sprintf(`name: paginate-dag-%d
steps:
  - name: step1
    command: echo "dag %d"`, i, i)
		err := store.Create(ctx, fmt.Sprintf("paginate-dag-%d", i), []byte(dagContent))
		require.NoError(t, err)
	}

	// Test pagination
	paginator := models.NewPaginator(2, 2) // limit=2, offset=2
	opts := models.ListDAGsOptions{Paginator: &paginator}
	result, errList, err := store.List(ctx, opts)
	require.NoError(t, err)
	require.Empty(t, errList)

	assert.Equal(t, 5, result.TotalCount)
	assert.Equal(t, 2, len(result.Items))
	assert.True(t, result.HasNextPage)
	assert.True(t, result.HasPrevPage)
}

func TestListWithFiltering(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-list-filtering")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	store := New(tmpDir)
	ctx := context.Background()

	// Create DAGs with different names and tags
	dag1Content := `name: filter-web-dag
tags: ["web", "frontend"]
steps:
  - name: step1
    command: echo "web"`
	err := store.Create(ctx, "filter-web-dag", []byte(dag1Content))
	require.NoError(t, err)

	dag2Content := `name: filter-batch-dag
tags: ["batch", "backend"]
steps:
  - name: step1
    command: echo "batch"`
	err = store.Create(ctx, "filter-batch-dag", []byte(dag2Content))
	require.NoError(t, err)

	// Test name filtering
	opts := models.ListDAGsOptions{Name: "web"}
	result, errList, err := store.List(ctx, opts)
	require.NoError(t, err)
	require.Empty(t, errList)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "filter-web-dag", result.Items[0].Name)

	// Test tag filtering
	opts = models.ListDAGsOptions{Tag: "frontend"}
	result, errList, err = store.List(ctx, opts)
	require.NoError(t, err)
	require.Empty(t, errList)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "filter-web-dag", result.Items[0].Name)

	// Test case-insensitive tag filtering
	opts = models.ListDAGsOptions{Tag: "FRONTEND"}
	result, errList, err = store.List(ctx, opts)
	require.NoError(t, err)
	require.Empty(t, errList)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "filter-web-dag", result.Items[0].Name)
}
