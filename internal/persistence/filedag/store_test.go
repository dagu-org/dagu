package filedag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// Register executor capabilities for testing.
	// In production, this is done by runtime/builtin init functions.
	for _, t := range []string{"", "shell", "command"} {
		core.RegisterExecutorCapabilities(t, core.ExecutorCapabilities{
			Command: true, MultipleCommands: true, Script: true, Shell: true,
		})
	}
	os.Exit(m.Run())
}

func TestStore(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-suspend-checker")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	dr := New(tmpDir, WithSkipExamples(true))
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

	store := New(tmpDir, WithSkipExamples(true))
	ctx := context.Background()

	// Create DAG files in different directory levels
	// Root level DAG
	rootDAG := `name: root-dag
steps:
  - name: step1
    command: echo "root"`
	err := os.WriteFile(filepath.Join(tmpDir, "root-dag.yaml"), []byte(rootDAG), 0600)
	require.NoError(t, err)

	// Create subdirectory and DAG
	subDir := filepath.Join(tmpDir, "subdir")
	err = os.MkdirAll(subDir, 0750)
	require.NoError(t, err)

	subDAG := `name: sub-dag
steps:
  - name: step1
    command: echo "sub"`
	err = os.WriteFile(filepath.Join(subDir, "sub-dag.yaml"), []byte(subDAG), 0600)
	require.NoError(t, err)

	// Create nested subdirectory and DAG
	nestedDir := filepath.Join(tmpDir, "subdir", "nested")
	err = os.MkdirAll(nestedDir, 0750)
	require.NoError(t, err)

	nestedDAG := `name: nested-dag
steps:
  - name: step1
    command: echo "nested"`
	err = os.WriteFile(filepath.Join(nestedDir, "nested-dag.yaml"), []byte(nestedDAG), 0600)
	require.NoError(t, err)

	// List all DAGs
	opts := execution.ListDAGsOptions{}
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

	store := New(tmpDir, WithSkipExamples(true))
	ctx := context.Background()

	// Test successful metadata retrieval
	dagContent := `name: test-dag
tags: ["tag1", "tag2"]
steps:
  - name: step1
    command: echo "hello"`
	err := os.WriteFile(filepath.Join(tmpDir, "test-dag.yaml"), []byte(dagContent), 0600)
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

	store := New(tmpDir, WithSkipExamples(true))
	ctx := context.Background()

	// Test successful details retrieval
	dagContent := `name: detailed-dag
schedule: "0 1 * * *"
steps:
  - name: step1
    command: echo "detailed"`
	err := os.WriteFile(filepath.Join(tmpDir, "detailed-dag.yaml"), []byte(dagContent), 0600)
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

	store := New(tmpDir, WithSkipExamples(true))
	ctx := context.Background()

	// Test successful spec retrieval
	dagContent := `name: spec-dag
steps:
  - name: step1
    command: echo "spec"`
	err := os.WriteFile(filepath.Join(tmpDir, "spec-dag.yaml"), []byte(dagContent), 0600)
	require.NoError(t, err)

	spec, err := store.GetSpec(ctx, "spec-dag")
	require.NoError(t, err)
	assert.Equal(t, dagContent, spec)

	// Test DAG not found
	_, err = store.GetSpec(ctx, "non-existent")
	require.Error(t, err)
	assert.Equal(t, execution.ErrDAGNotFound, err)
}

func TestCreate(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-create")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	store := New(tmpDir, WithSkipExamples(true))
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
	assert.Equal(t, execution.ErrDAGAlreadyExists, err)
}

func TestUpdateSpec(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-update-spec")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	store := New(tmpDir, WithSkipExamples(true))
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

	store := New(tmpDir, WithSkipExamples(true))
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

	store := New(tmpDir, WithSkipExamples(true))
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
	assert.Equal(t, execution.ErrDAGAlreadyExists, err)
}

func TestGrep(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-grep")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	store := New(tmpDir, WithSkipExamples(true))
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

	store := New(tmpDir, WithSkipExamples(true))
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

	store := New(tmpDir, WithSkipExamples(true))
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

	store := New(tmpDir, WithSkipExamples(true))
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
	paginator := execution.NewPaginator(2, 2) // limit=2, offset=2
	opts := execution.ListDAGsOptions{Paginator: &paginator}
	result, errList, err := store.List(ctx, opts)
	require.NoError(t, err)
	require.Empty(t, errList)

	assert.Equal(t, 5, result.TotalCount)
	assert.Equal(t, 2, len(result.Items))
	assert.True(t, result.HasNextPage)
	assert.True(t, result.HasPrevPage)
}

func TestListAlphabeticalSorting(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-list-alphabetical")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	store := New(tmpDir, WithSkipExamples(true))
	ctx := context.Background()

	// Create DAGs in non-alphabetical order
	dags := []struct {
		fileName string
		name     string
	}{
		{"zebra-dag", "zebra-dag"},
		{"alpha-dag", "alpha-dag"},
		{"beta-dag", "beta-dag"},
		{"CAPITAL-dag", "CAPITAL-dag"},
		{"123-numeric-dag", "123-numeric-dag"},
	}

	for _, dag := range dags {
		content := fmt.Sprintf(`name: %s
steps:
  - name: step1
    command: echo "%s"`, dag.name, dag.name)
		err := store.Create(ctx, dag.fileName, []byte(content))
		require.NoError(t, err)
	}

	// List all DAGs
	opts := execution.ListDAGsOptions{}
	result, errList, err := store.List(ctx, opts)
	require.NoError(t, err)
	require.Empty(t, errList)
	require.Len(t, result.Items, 5)

	// Verify alphabetical order (case-insensitive)
	expectedOrder := []string{
		"123-numeric-dag",
		"alpha-dag",
		"beta-dag",
		"CAPITAL-dag",
		"zebra-dag",
	}

	for i, dag := range result.Items {
		assert.Equal(t, expectedOrder[i], dag.Name,
			"DAG at position %d should be %s but was %s",
			i, expectedOrder[i], dag.Name)
	}
}

func TestListWithFiltering(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-list-filtering")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	store := New(tmpDir, WithSkipExamples(true))
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
	opts := execution.ListDAGsOptions{Name: "web"}
	result, errList, err := store.List(ctx, opts)
	require.NoError(t, err)
	require.Empty(t, errList)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "filter-web-dag", result.Items[0].Name)

	// Test tag filtering
	opts = execution.ListDAGsOptions{Tag: "frontend"}
	result, errList, err = store.List(ctx, opts)
	require.NoError(t, err)
	require.Empty(t, errList)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "filter-web-dag", result.Items[0].Name)

	// Test case-insensitive tag filtering
	opts = execution.ListDAGsOptions{Tag: "FRONTEND"}
	result, errList, err = store.List(ctx, opts)
	require.NoError(t, err)
	require.Empty(t, errList)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "filter-web-dag", result.Items[0].Name)
}

func TestListWithSortAndOrder(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-list-sort-order")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	store := New(tmpDir, WithSkipExamples(true))
	ctx := context.Background()

	// Create DAGs with different names
	dags := []struct {
		fileName string
		name     string
	}{
		{"zebra-dag", "zebra-dag"},
		{"alpha-dag", "alpha-dag"},
		{"beta-dag", "beta-dag"},
		{"GAMMA-dag", "GAMMA-dag"},
	}

	// Create files with some time delay to ensure different modification times
	for i, dag := range dags {
		content := fmt.Sprintf(`name: %s
steps:
  - name: step1
    command: echo "%s"`, dag.name, dag.name)
		err := store.Create(ctx, dag.fileName, []byte(content))
		require.NoError(t, err)

		// Add a small delay between file creations to ensure different mod times
		if i < len(dags)-1 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Test 1: Sort by name ascending (default)
	opts := execution.ListDAGsOptions{
		Sort:  "name",
		Order: "asc",
	}
	result, errList, err := store.List(ctx, opts)
	require.NoError(t, err)
	require.Empty(t, errList)
	require.Len(t, result.Items, 4)

	assert.Equal(t, "alpha-dag", result.Items[0].Name)
	assert.Equal(t, "beta-dag", result.Items[1].Name)
	assert.Equal(t, "GAMMA-dag", result.Items[2].Name)
	assert.Equal(t, "zebra-dag", result.Items[3].Name)

	// Test 2: Sort by name descending
	opts = execution.ListDAGsOptions{
		Sort:  "name",
		Order: "desc",
	}
	result, errList, err = store.List(ctx, opts)
	require.NoError(t, err)
	require.Empty(t, errList)
	require.Len(t, result.Items, 4)

	assert.Equal(t, "zebra-dag", result.Items[0].Name)
	assert.Equal(t, "GAMMA-dag", result.Items[1].Name)
	assert.Equal(t, "beta-dag", result.Items[2].Name)
	assert.Equal(t, "alpha-dag", result.Items[3].Name)

	// Test 3: Sort by updated_at should fall back to name sorting in storage layer
	opts = execution.ListDAGsOptions{
		Sort:  "updated_at",
		Order: "asc",
	}
	result, errList, err = store.List(ctx, opts)
	require.NoError(t, err)
	require.Empty(t, errList)
	require.Len(t, result.Items, 4)

	// Should be sorted by name since updated_at is handled in API layer
	assert.Equal(t, "alpha-dag", result.Items[0].Name)
	assert.Equal(t, "beta-dag", result.Items[1].Name)
	assert.Equal(t, "GAMMA-dag", result.Items[2].Name)
	assert.Equal(t, "zebra-dag", result.Items[3].Name)

	// Test 4: Sort by updated_at desc should also fall back to name
	opts = execution.ListDAGsOptions{
		Sort:  "updated_at",
		Order: "desc",
	}
	result, errList, err = store.List(ctx, opts)
	require.NoError(t, err)
	require.Empty(t, errList)
	require.Len(t, result.Items, 4)

	// Should be sorted by name descending (fallback behavior)
	assert.Equal(t, "zebra-dag", result.Items[0].Name)
	assert.Equal(t, "GAMMA-dag", result.Items[1].Name)
	assert.Equal(t, "beta-dag", result.Items[2].Name)
	assert.Equal(t, "alpha-dag", result.Items[3].Name)

	// Test 5: Default sort (empty sort field) should sort by name
	opts = execution.ListDAGsOptions{
		Sort:  "",
		Order: "asc",
	}
	result, errList, err = store.List(ctx, opts)
	require.NoError(t, err)
	require.Empty(t, errList)
	require.Len(t, result.Items, 4)

	assert.Equal(t, "alpha-dag", result.Items[0].Name)
	assert.Equal(t, "beta-dag", result.Items[1].Name)
	assert.Equal(t, "GAMMA-dag", result.Items[2].Name)
	assert.Equal(t, "zebra-dag", result.Items[3].Name)

	// Test 6: Unknown sort field falls back to name
	opts = execution.ListDAGsOptions{
		Sort:  "unknown",
		Order: "asc",
	}
	result, errList, err = store.List(ctx, opts)
	require.NoError(t, err)
	require.Empty(t, errList)
	require.Len(t, result.Items, 4)

	assert.Equal(t, "alpha-dag", result.Items[0].Name)
	assert.Equal(t, "beta-dag", result.Items[1].Name)
	assert.Equal(t, "GAMMA-dag", result.Items[2].Name)
	assert.Equal(t, "zebra-dag", result.Items[3].Name)
}

func TestListWithSortingAndPagination(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-list-sort-pagination")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	store := New(tmpDir, WithSkipExamples(true))
	ctx := context.Background()

	// Create multiple DAGs for pagination testing
	dagNames := []string{
		"zulu-dag", "yankee-dag", "x-ray-dag", "whiskey-dag",
		"victor-dag", "uniform-dag", "tango-dag", "sierra-dag",
		"romeo-dag", "quebec-dag", "papa-dag", "oscar-dag",
	}

	for i, name := range dagNames {
		content := fmt.Sprintf(`name: %s
steps:
  - name: step1
    command: echo "%s"`, name, name)
		err := store.Create(ctx, name, []byte(content))
		require.NoError(t, err)

		// Add delay to ensure different mod times
		if i < len(dagNames)-1 {
			time.Sleep(5 * time.Millisecond)
		}
	}

	// Test 1: Name sort ascending with pagination
	// Page 1
	paginator := execution.NewPaginator(1, 5) // page=1, perPage=5
	opts := execution.ListDAGsOptions{
		Paginator: &paginator,
		Sort:      "name",
		Order:     "asc",
	}
	result, errList, err := store.List(ctx, opts)
	require.NoError(t, err)
	require.Empty(t, errList)
	assert.Equal(t, 12, result.TotalCount)
	assert.Len(t, result.Items, 5)
	assert.True(t, result.HasNextPage)
	assert.False(t, result.HasPrevPage)

	// First page should have first 5 alphabetically
	expectedPage1 := []string{"oscar-dag", "papa-dag", "quebec-dag", "romeo-dag", "sierra-dag"}
	for i, expected := range expectedPage1 {
		assert.Equal(t, expected, result.Items[i].Name)
	}

	// Page 2
	paginator = execution.NewPaginator(2, 5) // page=2, perPage=5
	opts.Paginator = &paginator
	result, errList, err = store.List(ctx, opts)
	require.NoError(t, err)
	require.Empty(t, errList)
	assert.Len(t, result.Items, 5)
	assert.True(t, result.HasNextPage)
	assert.True(t, result.HasPrevPage)

	// Second page should have next 5 alphabetically
	expectedPage2 := []string{"tango-dag", "uniform-dag", "victor-dag", "whiskey-dag", "x-ray-dag"}
	for i, expected := range expectedPage2 {
		assert.Equal(t, expected, result.Items[i].Name)
	}

	// Page 3
	paginator = execution.NewPaginator(3, 5) // page=3, perPage=5
	opts.Paginator = &paginator
	result, errList, err = store.List(ctx, opts)
	require.NoError(t, err)
	require.Empty(t, errList)
	assert.Len(t, result.Items, 2) // Only 2 items left
	assert.False(t, result.HasNextPage)
	assert.True(t, result.HasPrevPage)

	// Third page should have last 2 alphabetically
	assert.Equal(t, "yankee-dag", result.Items[0].Name)
	assert.Equal(t, "zulu-dag", result.Items[1].Name)

	// Test 2: Name sort descending with pagination
	paginator = execution.NewPaginator(1, 5) // page=1, perPage=5
	opts = execution.ListDAGsOptions{
		Paginator: &paginator,
		Sort:      "name",
		Order:     "desc",
	}
	result, errList, err = store.List(ctx, opts)
	require.NoError(t, err)
	require.Empty(t, errList)
	assert.Len(t, result.Items, 5)

	// First page descending should start with zulu
	expectedDesc := []string{"zulu-dag", "yankee-dag", "x-ray-dag", "whiskey-dag", "victor-dag"}
	for i, expected := range expectedDesc {
		assert.Equal(t, expected, result.Items[i].Name)
	}

	// Test 3: Non-name sort fields fall back to name sorting in storage layer
	paginator = execution.NewPaginator(1, 5) // page=1, perPage=5
	opts = execution.ListDAGsOptions{
		Paginator: &paginator,
		Sort:      "updated_at",
		Order:     "desc", // This will fall back to name desc
	}
	result, errList, err = store.List(ctx, opts)
	require.NoError(t, err)
	require.Empty(t, errList)
	assert.Len(t, result.Items, 5)

	// Should be sorted by name descending (fallback)
	expectedDesc = []string{"zulu-dag", "yankee-dag", "x-ray-dag", "whiskey-dag", "victor-dag"}
	for i, expected := range expectedDesc {
		assert.Equal(t, expected, result.Items[i].Name)
	}
}

func TestListIncludesDAGsWithErrors(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir, WithSkipExamples(true))
	ctx := context.Background()

	// Create a valid DAG
	validDAG := `
steps:
  - name: step1
    command: echo hello
`
	err := os.WriteFile(filepath.Join(tmpDir, "valid.yaml"), []byte(validDAG), 0644)
	require.NoError(t, err)

	// Create a DAG with errors (references non-existent step)
	errorDAG := `
nonexistent: error-dag
steps:
  - name: step1
    command: echo hello
`
	err = os.WriteFile(filepath.Join(tmpDir, "error.yaml"), []byte(errorDAG), 0644)
	require.NoError(t, err)

	// List all DAGs
	result, errList, err := store.List(ctx, execution.ListDAGsOptions{})
	require.NoError(t, err)

	// Should include both DAGs
	assert.Equal(t, 2, len(result.Items))

	// Check that both DAGs are present
	dagNames := make(map[string]bool)
	hasErrors := false
	for _, dag := range result.Items {
		t.Logf("Found DAG: %s, BuildErrors: %v", dag.Name, dag.BuildErrors)
		dagNames[dag.Name] = true
		if len(dag.BuildErrors) > 0 {
			hasErrors = true
		}
	}

	assert.True(t, dagNames["valid"], "valid should be in the list")
	assert.True(t, dagNames["error"], "error should be in the list")
	assert.True(t, hasErrors, "At least one DAG should have build errors")

	// Error list might contain warnings but should not fail
	t.Logf("Error list: %v", errList)
}

func TestListWithNextRunSorting(t *testing.T) {
	tmpDir := fileutil.MustTempDir("test-list-nextrun-sort")
	t.Cleanup(func() {
		_ = os.RemoveAll(tmpDir)
	})

	store := New(tmpDir, WithSkipExamples(true))
	ctx := context.Background()

	// Create test DAG files directly
	createDAG := func(name, schedule string) {
		content := fmt.Sprintf("name: %s\nsteps:\n  - echo test", name)
		if schedule != "" {
			content = fmt.Sprintf("name: %s\nschedule: %s\nsteps:\n  - echo test", name, schedule)
		}
		err := os.WriteFile(filepath.Join(tmpDir, name+".yaml"), []byte(content), 0600)
		require.NoError(t, err)
	}

	// Create DAGs with different schedules
	createDAG("hourly-dag", "\"0 * * * *\"") // Runs at the top of every hour
	createDAG("daily-dag", "\"0 1 * * *\"")  // Runs at 01:00 every day
	createDAG("no-schedule", "")

	// Use a fixed time for deterministic testing: 2024-01-15 01:30:00
	// At this time:
	// - hourly-dag next run: 2024-01-15 02:00:00 (30 minutes away)
	// - daily-dag next run: 2024-01-16 01:00:00 (23.5 hours away)
	fixedTime := time.Date(2024, 1, 15, 1, 30, 0, 0, time.UTC)

	// Test ascending order
	result, _, err := store.List(ctx, execution.ListDAGsOptions{
		Sort:  "nextRun",
		Order: "asc",
		Time:  &fixedTime,
	})
	require.NoError(t, err)
	require.Len(t, result.Items, 3)

	// Verify order: hourly (runs soonest) -> daily -> no-schedule (last)
	assert.Equal(t, "hourly-dag", result.Items[0].Name)
	assert.Equal(t, "daily-dag", result.Items[1].Name)
	assert.Equal(t, "no-schedule", result.Items[2].Name)

	// Test descending order
	result, _, err = store.List(ctx, execution.ListDAGsOptions{
		Sort:  "nextRun",
		Order: "desc",
		Time:  &fixedTime,
	})
	require.NoError(t, err)
	require.Len(t, result.Items, 3)

	// Verify order: daily (runs latest) -> hourly -> no-schedule (still last)
	// Note: no-schedule always comes last because IsZero() time is treated as greater
	assert.Equal(t, "daily-dag", result.Items[0].Name)
	assert.Equal(t, "hourly-dag", result.Items[1].Name)
	assert.Equal(t, "no-schedule", result.Items[2].Name)
}
