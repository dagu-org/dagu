package filedag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorage_ListWithPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	ctx := context.Background()

	// Create test directory structure
	testDirs := []string{
		"",
		"workflow",
		"workflow/extract",
		"data",
		"data/pipeline",
	}

	for _, dir := range testDirs {
		if dir != "" {
			err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755)
			require.NoError(t, err)
		}
	}

	// Create test DAG files
	testDAGs := []struct {
		path string
		spec string
	}{
		{
			path: "root1.yaml",
			spec: `name: root1
steps:
  - name: step1
    command: echo "root1"`,
		},
		{
			path: "root2.yaml",
			spec: `name: root2
steps:
  - name: step1
    command: echo "root2"`,
		},
		{
			path: "workflow/task1.yaml",
			spec: `name: task1
steps:
  - name: step1
    command: echo "workflow task1"`,
		},
		{
			path: "workflow/task2.yaml",
			spec: `name: task2
steps:
  - name: step1
    command: echo "workflow task2"`,
		},
		{
			path: "workflow/extract/users.yaml",
			spec: `name: users
steps:
  - name: step1
    command: echo "extract users"`,
		},
		{
			path: "data/process.yaml",
			spec: `name: process
steps:
  - name: step1
    command: echo "data process"`,
		},
		{
			path: "data/pipeline/transform.yaml",
			spec: `name: transform
steps:
  - name: step1
    command: echo "pipeline transform"`,
		},
	}

	// Create DAG files
	for _, testDAG := range testDAGs {
		filePath := filepath.Join(tmpDir, testDAG.path)
		err := os.WriteFile(filePath, []byte(testDAG.spec), 0644)
		require.NoError(t, err)
	}

	tests := []struct {
		name              string
		prefix            string
		expectedDAGNames  []string
		expectedSubdirs   []string
		expectedCount     int
	}{
		{
			name:             "list root directory",
			prefix:           "",
			expectedDAGNames: []string{"root1", "root2"},
			expectedSubdirs:  []string{"data", "workflow"},
			expectedCount:    2,
		},
		{
			name:             "list workflow directory",
			prefix:           "workflow",
			expectedDAGNames: []string{"workflow/task1", "workflow/task2"},
			expectedSubdirs:  []string{"extract"},
			expectedCount:    2,
		},
		{
			name:             "list workflow/extract directory",
			prefix:           "workflow/extract",
			expectedDAGNames: []string{"workflow/extract/users"},
			expectedSubdirs:  []string{},
			expectedCount:    1,
		},
		{
			name:             "list data directory",
			prefix:           "data",
			expectedDAGNames: []string{"data/process"},
			expectedSubdirs:  []string{"pipeline"},
			expectedCount:    1,
		},
		{
			name:             "list data/pipeline directory",
			prefix:           "data/pipeline",
			expectedDAGNames: []string{"data/pipeline/transform"},
			expectedSubdirs:  []string{},
			expectedCount:    1,
		},
		{
			name:             "list non-existent directory",
			prefix:           "nonexistent",
			expectedDAGNames: []string{},
			expectedSubdirs:  []string{},
			expectedCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := models.ListDAGsOptions{}
			result, subdirs, errs, err := store.ListWithPrefix(ctx, tt.prefix, opts)
			
			require.NoError(t, err)
			assert.Empty(t, errs)
			assert.Equal(t, tt.expectedCount, result.TotalCount)
			assert.Len(t, result.Items, tt.expectedCount)
			
			// Check DAG names
			var actualNames []string
			for _, dag := range result.Items {
				actualNames = append(actualNames, dag.FileName())
			}
			assert.ElementsMatch(t, tt.expectedDAGNames, actualNames)
			
			// Check subdirectories
			assert.ElementsMatch(t, tt.expectedSubdirs, subdirs)
		})
	}
}

func TestStorage_CreateWithPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	ctx := context.Background()

	tests := []struct {
		name     string
		dagName  string
		spec     string
		wantErr  bool
	}{
		{
			name:    "create DAG at root",
			dagName: "root_dag",
			spec: `name: root_dag
steps:
  - name: step1
    command: echo "hello"`,
			wantErr: false,
		},
		{
			name:    "create DAG with single level prefix",
			dagName: "workflow/task1",
			spec: `name: task1
steps:
  - name: step1
    command: echo "hello"`,
			wantErr: false,
		},
		{
			name:    "create DAG with nested prefix",
			dagName: "data/pipeline/extract/users",
			spec: `name: users
steps:
  - name: step1
    command: echo "hello"`,
			wantErr: false,
		},
		{
			name:    "create DAG with invalid spec",
			dagName: "invalid/dag",
			spec:    `invalid yaml content`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.Create(ctx, tt.dagName, []byte(tt.spec))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				
				// Verify the file was created
				expectedPath := filepath.Join(tmpDir, tt.dagName+".yaml")
				assert.FileExists(t, expectedPath)
				
				// Verify we can retrieve it
				dag, err := store.GetMetadata(ctx, tt.dagName)
				assert.NoError(t, err)
				assert.NotNil(t, dag)
				assert.Equal(t, tt.dagName, dag.FileName())
			}
		})
	}
}

func TestStorage_GetMetadataWithPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	ctx := context.Background()

	// Create test structure
	testDAGs := []struct {
		name string
		spec string
	}{
		{
			name: "simple",
			spec: `name: simple
steps:
  - name: step1
    command: echo "simple"`,
		},
		{
			name: "workflow/task1",
			spec: `name: task1
steps:
  - name: step1
    command: echo "task1"`,
		},
		{
			name: "data/pipeline/process",
			spec: `name: process
steps:
  - name: step1
    command: echo "process"`,
		},
	}

	// Create DAGs
	for _, td := range testDAGs {
		err := store.Create(ctx, td.name, []byte(td.spec))
		require.NoError(t, err)
	}

	// Test retrieval
	for _, td := range testDAGs {
		t.Run("get "+td.name, func(t *testing.T) {
			dag, err := store.GetMetadata(ctx, td.name)
			require.NoError(t, err)
			assert.NotNil(t, dag)
			assert.Equal(t, td.name, dag.FileName())
		})
	}

	// Test non-existent DAG
	t.Run("get non-existent", func(t *testing.T) {
		_, err := store.GetMetadata(ctx, "nonexistent/dag")
		assert.Error(t, err)
	})
}

func TestStorage_DeleteWithPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	ctx := context.Background()

	// Create a DAG with prefix
	dagName := "workflow/temp_task"
	spec := `name: temp_task
steps:
  - name: step1
    command: echo "temp"`
	
	err := store.Create(ctx, dagName, []byte(spec))
	require.NoError(t, err)

	// Verify it exists
	dag, err := store.GetMetadata(ctx, dagName)
	require.NoError(t, err)
	assert.NotNil(t, dag)

	// Delete it
	err = store.Delete(ctx, dagName)
	assert.NoError(t, err)

	// Verify it's gone
	_, err = store.GetMetadata(ctx, dagName)
	assert.Error(t, err)
}

func TestStorage_RenameWithPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	ctx := context.Background()

	tests := []struct {
		name    string
		oldName string
		newName string
		spec    string
		wantErr bool
	}{
		{
			name:    "rename within same directory",
			oldName: "workflow/old_task",
			newName: "workflow/new_task",
			spec: `name: old_task
steps:
  - name: step1
    command: echo "task"`,
			wantErr: false,
		},
		{
			name:    "rename to different directory",
			oldName: "data/old_process",
			newName: "archive/old_process",
			spec: `name: old_process
steps:
  - name: step1
    command: echo "process"`,
			wantErr: false,
		},
		{
			name:    "rename from root to directory",
			oldName: "root_dag",
			newName: "organized/root_dag",
			spec: `name: root_dag
steps:
  - name: step1
    command: echo "root"`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create original DAG
			err := store.Create(ctx, tt.oldName, []byte(tt.spec))
			require.NoError(t, err)

			// Rename it
			err = store.Rename(ctx, tt.oldName, tt.newName)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify old name no longer exists
				_, err = store.GetMetadata(ctx, tt.oldName)
				assert.Error(t, err)

				// Verify new name exists
				dag, err := store.GetMetadata(ctx, tt.newName)
				assert.NoError(t, err)
				assert.NotNil(t, dag)
				assert.Equal(t, tt.newName, dag.FileName())
			}
		})
	}
}

func TestStorage_GrepWithPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	ctx := context.Background()

	// Create test DAGs with searchable content
	testDAGs := []struct {
		name string
		spec string
	}{
		{
			name: "root_search",
			spec: `name: root_search
description: This has FINDME in the root
steps:
  - name: step1
    command: echo "root"`,
		},
		{
			name: "workflow/task_search",
			spec: `name: task_search
steps:
  - name: step1
    command: echo "FINDME in workflow"`,
		},
		{
			name: "data/pipeline/deep_search",
			spec: `name: deep_search
steps:
  - name: findme_step
    command: echo "Deep nested FINDME"`,
		},
		{
			name: "no_match",
			spec: `name: no_match
steps:
  - name: step1
    command: echo "nothing here"`,
		},
	}

	// Create DAGs
	for _, td := range testDAGs {
		err := store.Create(ctx, td.name, []byte(td.spec))
		require.NoError(t, err)
	}

	// Search for pattern
	results, errs, err := store.Grep(ctx, "FINDME")
	require.NoError(t, err)
	assert.Empty(t, errs)
	assert.Len(t, results, 3)

	// Verify results include prefixed DAGs
	foundNames := make(map[string]bool)
	for _, result := range results {
		foundNames[result.Name] = true
	}

	assert.True(t, foundNames["root_search"])
	assert.True(t, foundNames["workflow/task_search"])
	assert.True(t, foundNames["data/pipeline/deep_search"])
	assert.False(t, foundNames["no_match"])
}

func TestStorage_TagListWithPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	ctx := context.Background()

	// Create test DAGs with tags
	testDAGs := []struct {
		name string
		spec string
	}{
		{
			name: "root_tagged",
			spec: `name: root_tagged
tags: [production, critical]
steps:
  - name: step1
    command: echo "root"`,
		},
		{
			name: "workflow/dev_task",
			spec: `name: dev_task
tags: [development, testing]
steps:
  - name: step1
    command: echo "dev"`,
		},
		{
			name: "data/pipeline/batch",
			spec: `name: batch
tags: [production, batch-job]
steps:
  - name: step1
    command: echo "batch"`,
		},
	}

	// Create DAGs
	for _, td := range testDAGs {
		err := store.Create(ctx, td.name, []byte(td.spec))
		require.NoError(t, err)
	}

	// Get all tags
	tags, errs, err := store.TagList(ctx)
	require.NoError(t, err)
	assert.Empty(t, errs)

	// Should have all unique tags
	expectedTags := []string{"production", "critical", "development", "testing", "batch-job"}
	assert.ElementsMatch(t, expectedTags, tags)
}

func TestStorage_ListWithPrefixPagination(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	ctx := context.Background()

	// Create many DAGs in a directory
	prefix := "paginated"
	err := os.MkdirAll(filepath.Join(tmpDir, prefix), 0755)
	require.NoError(t, err)

	// Create 15 DAGs
	for i := 1; i <= 15; i++ {
		dagName := filepath.Join(prefix, fmt.Sprintf("task%02d", i))
		spec := fmt.Sprintf(`name: task%02d
steps:
  - name: step1
    command: echo "task %d"`, i, i)
		err := store.Create(ctx, dagName, []byte(spec))
		require.NoError(t, err)
	}

	// Test pagination
	tests := []struct {
		name          string
		page          int
		pageSize      int
		expectedCount int
		expectedTotal int
	}{
		{
			name:          "first page",
			page:          1,
			pageSize:      5,
			expectedCount: 5,
			expectedTotal: 15,
		},
		{
			name:          "second page",
			page:          2,
			pageSize:      5,
			expectedCount: 5,
			expectedTotal: 15,
		},
		{
			name:          "last page",
			page:          3,
			pageSize:      5,
			expectedCount: 5,
			expectedTotal: 15,
		},
		{
			name:          "beyond last page",
			page:          4,
			pageSize:      5,
			expectedCount: 0,
			expectedTotal: 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paginator := models.NewPaginator(tt.page, tt.pageSize)
			opts := models.ListDAGsOptions{
				Paginator: &paginator,
			}
			
			result, _, errs, err := store.ListWithPrefix(ctx, prefix, opts)
			require.NoError(t, err)
			assert.Empty(t, errs)
			assert.Equal(t, tt.expectedTotal, result.TotalCount)
			assert.Len(t, result.Items, tt.expectedCount)
		})
	}
}

func TestStorage_ListWithPrefixFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	ctx := context.Background()

	// Create test DAGs with various attributes
	testDAGs := []struct {
		name string
		spec string
	}{
		{
			name: "filter/production_api",
			spec: `name: production_api
tags: [production, api]
steps:
  - name: step1
    command: echo "prod api"`,
		},
		{
			name: "filter/development_api",
			spec: `name: development_api
tags: [development, api]
steps:
  - name: step1
    command: echo "dev api"`,
		},
		{
			name: "filter/production_batch",
			spec: `name: production_batch
tags: [production, batch]
steps:
  - name: step1
    command: echo "prod batch"`,
		},
	}

	// Create DAGs
	for _, td := range testDAGs {
		err := store.Create(ctx, td.name, []byte(td.spec))
		require.NoError(t, err)
	}

	// Test name filtering
	t.Run("filter by name", func(t *testing.T) {
		opts := models.ListDAGsOptions{
			Name: "api",
		}
		result, _, errs, err := store.ListWithPrefix(ctx, "filter", opts)
		require.NoError(t, err)
		assert.Empty(t, errs)
		assert.Equal(t, 2, result.TotalCount)
	})

	// Test tag filtering
	t.Run("filter by tag", func(t *testing.T) {
		opts := models.ListDAGsOptions{
			Tag: "production",
		}
		result, _, errs, err := store.ListWithPrefix(ctx, "filter", opts)
		require.NoError(t, err)
		assert.Empty(t, errs)
		assert.Equal(t, 2, result.TotalCount)
	})

	// Test combined filtering
	t.Run("filter by name and tag", func(t *testing.T) {
		opts := models.ListDAGsOptions{
			Name: "api",
			Tag:  "production",
		}
		result, _, errs, err := store.ListWithPrefix(ctx, "filter", opts)
		require.NoError(t, err)
		assert.Empty(t, errs)
		assert.Equal(t, 1, result.TotalCount)
		assert.Equal(t, "filter/production_api", result.Items[0].FileName())
	})
}

func TestStorage_UpdateSpecWithPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	ctx := context.Background()

	// Create a DAG with prefix
	dagName := "workflow/update_test"
	originalSpec := `name: update_test
steps:
  - name: step1
    command: echo "original"`
	
	err := store.Create(ctx, dagName, []byte(originalSpec))
	require.NoError(t, err)

	// Update the spec
	newSpec := `name: update_test
description: Updated description
steps:
  - name: step1
    command: echo "updated"
  - name: step2
    command: echo "new step"`

	err = store.UpdateSpec(ctx, dagName, []byte(newSpec))
	assert.NoError(t, err)

	// Verify the update
	dag, err := store.GetDetails(ctx, dagName)
	require.NoError(t, err)
	assert.Equal(t, "Updated description", dag.Description)
	assert.Len(t, dag.Steps, 2)
	assert.Equal(t, dagName, dag.FileName())
}

func TestStorage_ToggleSuspendWithPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir, WithFlagsBaseDir(filepath.Join(tmpDir, "flags")))
	ctx := context.Background()

	// Create a DAG with prefix
	dagName := "workflow/suspend_test"
	spec := `name: suspend_test
steps:
  - name: step1
    command: echo "test"`
	
	err := store.Create(ctx, dagName, []byte(spec))
	require.NoError(t, err)

	// Initially should not be suspended
	assert.False(t, store.IsSuspended(ctx, dagName))

	// Suspend it
	err = store.ToggleSuspend(ctx, dagName, true)
	assert.NoError(t, err)
	assert.True(t, store.IsSuspended(ctx, dagName))

	// Unsuspend it
	err = store.ToggleSuspend(ctx, dagName, false)
	assert.NoError(t, err)
	assert.False(t, store.IsSuspended(ctx, dagName))
}

func TestStorage_BackwardCompatibility(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	ctx := context.Background()

	// Create DAGs using the old List method
	rootDAG := "compatibility_test"
	spec := `name: compatibility_test
steps:
  - name: step1
    command: echo "test"`
	
	err := store.Create(ctx, rootDAG, []byte(spec))
	require.NoError(t, err)

	// The old List method should still work
	opts := models.ListDAGsOptions{}
	result, errs, err := store.List(ctx, opts)
	
	require.NoError(t, err)
	assert.Empty(t, errs)
	assert.Equal(t, 1, result.TotalCount)
	assert.Equal(t, rootDAG, result.Items[0].FileName())
}

func TestStorage_SearchPaths(t *testing.T) {
	tmpDir := t.TempDir()
	altDir := t.TempDir()
	
	// Create DAG in alternative directory
	altDAGPath := filepath.Join(altDir, "external.yaml")
	spec := `name: external
steps:
  - name: step1
    command: echo "external"`
	err := os.WriteFile(altDAGPath, []byte(spec), 0644)
	require.NoError(t, err)

	// Create store with search paths
	store := New(tmpDir, WithSearchPaths([]string{altDir}))
	ctx := context.Background()

	// Should be able to find DAG by name
	dag, err := store.GetMetadata(ctx, "external")
	require.NoError(t, err)
	assert.NotNil(t, dag)
	assert.Equal(t, "external", dag.FileName())
}