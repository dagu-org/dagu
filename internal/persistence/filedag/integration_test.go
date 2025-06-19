package filedag

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPrefixIntegration tests the full integration of prefix support
func TestPrefixIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	ctx := context.Background()

	// Step 1: Create DAGs in various directories
	dags := []struct {
		name string
		spec string
	}{
		{
			name: "daily_report",
			spec: `name: daily_report
description: Daily reporting task
tags: [production, reporting]
steps:
  - name: generate
    command: python generate_report.py`,
		},
		{
			name: "etl/extract_users",
			spec: `name: extract_users
description: Extract user data from source
tags: [etl, users]
steps:
  - name: extract
    command: python extract_users.py`,
		},
		{
			name: "etl/transform_users",
			spec: `name: transform_users
description: Transform user data
tags: [etl, users]
steps:
  - name: transform
    command: python transform_users.py`,
		},
		{
			name: "etl/load_users",
			spec: `name: load_users
description: Load user data to warehouse
tags: [etl, users]
steps:
  - name: load
    command: python load_users.py`,
		},
		{
			name: "monitoring/health_check",
			spec: `name: health_check
description: System health check
tags: [monitoring, critical]
steps:
  - name: check
    command: bash health_check.sh`,
		},
		{
			name: "monitoring/alerts/cpu_alert",
			spec: `name: cpu_alert
description: CPU usage alert
tags: [monitoring, alerts]
steps:
  - name: alert
    command: python cpu_alert.py`,
		},
	}

	// Create all DAGs
	for _, dag := range dags {
		err := store.Create(ctx, dag.name, []byte(dag.spec))
		require.NoError(t, err, "Failed to create DAG: %s", dag.name)
	}

	// Step 2: Test listing at root level
	t.Run("list root level", func(t *testing.T) {
		result, subdirs, errs, err := store.ListWithPrefix(ctx, "", models.ListDAGsOptions{})
		require.NoError(t, err)
		assert.Empty(t, errs)
		assert.Equal(t, 1, result.TotalCount) // Only daily_report
		assert.Len(t, subdirs, 2)              // etl and monitoring
		assert.ElementsMatch(t, []string{"etl", "monitoring"}, subdirs)
		assert.Equal(t, "daily_report", result.Items[0].FileName())
	})

	// Step 3: Test listing ETL directory
	t.Run("list etl directory", func(t *testing.T) {
		result, subdirs, errs, err := store.ListWithPrefix(ctx, "etl", models.ListDAGsOptions{})
		require.NoError(t, err)
		assert.Empty(t, errs)
		assert.Equal(t, 3, result.TotalCount) // 3 ETL DAGs
		assert.Empty(t, subdirs)
		
		names := make([]string, len(result.Items))
		for i, dag := range result.Items {
			names[i] = dag.FileName()
		}
		assert.ElementsMatch(t, []string{"etl/extract_users", "etl/transform_users", "etl/load_users"}, names)
	})

	// Step 4: Test listing monitoring directory
	t.Run("list monitoring directory", func(t *testing.T) {
		result, subdirs, errs, err := store.ListWithPrefix(ctx, "monitoring", models.ListDAGsOptions{})
		require.NoError(t, err)
		assert.Empty(t, errs)
		assert.Equal(t, 1, result.TotalCount) // Only health_check
		assert.Len(t, subdirs, 1)             // alerts subdirectory
		assert.Equal(t, "alerts", subdirs[0])
		assert.Equal(t, "monitoring/health_check", result.Items[0].FileName())
	})

	// Step 5: Test tag filtering
	t.Run("filter by etl tag in etl directory", func(t *testing.T) {
		opts := models.ListDAGsOptions{Tag: "etl"}
		result, _, errs, err := store.ListWithPrefix(ctx, "etl", opts)
		require.NoError(t, err)
		assert.Empty(t, errs)
		assert.Equal(t, 3, result.TotalCount) // All ETL DAGs have this tag
	})

	// Step 6: Test grep functionality
	t.Run("grep across all directories", func(t *testing.T) {
		results, errs, err := store.Grep(ctx, "python")
		require.NoError(t, err)
		assert.Empty(t, errs)
		assert.Len(t, results, 5) // All except health_check use python
		
		foundNames := make(map[string]bool)
		for _, result := range results {
			foundNames[result.Name] = true
		}
		assert.True(t, foundNames["daily_report"])
		assert.True(t, foundNames["etl/extract_users"])
		assert.True(t, foundNames["etl/transform_users"])
		assert.True(t, foundNames["etl/load_users"])
		assert.True(t, foundNames["monitoring/alerts/cpu_alert"])
		assert.False(t, foundNames["monitoring/health_check"])
	})

	// Step 7: Test renaming across directories
	t.Run("rename across directories", func(t *testing.T) {
		err := store.Rename(ctx, "daily_report", "archived/2024/daily_report")
		require.NoError(t, err)
		
		// Verify old name doesn't exist
		_, err = store.GetMetadata(ctx, "daily_report")
		assert.Error(t, err)
		
		// Verify new name exists
		dag, err := store.GetMetadata(ctx, "archived/2024/daily_report")
		require.NoError(t, err)
		assert.Equal(t, "archived/2024/daily_report", dag.FileName())
		assert.Equal(t, "daily_report", dag.Name)
	})

	// Step 8: Test backward compatibility
	t.Run("backward compatibility", func(t *testing.T) {
		// The old List method should still work for root-level DAGs
		result, errs, err := store.List(ctx, models.ListDAGsOptions{})
		require.NoError(t, err)
		assert.Empty(t, errs)
		// Should only see root-level DAGs (none after we moved daily_report)
		assert.Equal(t, 0, result.TotalCount)
	})

	// Step 9: Test validation
	t.Run("create with invalid path", func(t *testing.T) {
		// Try to create with invalid characters
		err := store.Create(ctx, "invalid/../dag", []byte(`name: test
steps:
  - name: step1
    command: echo test`))
		assert.NoError(t, err) // The store doesn't validate paths, but the file system might

		// Verify it can be retrieved with the exact name
		dag, err := store.GetMetadata(ctx, "invalid/../dag")
		assert.NoError(t, err)
		assert.NotNil(t, dag)
	})

	// Step 10: Test suspension with prefixed DAGs
	t.Run("suspend prefixed DAG", func(t *testing.T) {
		dagName := "etl/extract_users"
		
		// Initially not suspended
		assert.False(t, store.IsSuspended(ctx, dagName))
		
		// Suspend it
		err := store.ToggleSuspend(ctx, dagName, true)
		require.NoError(t, err)
		assert.True(t, store.IsSuspended(ctx, dagName))
		
		// Verify it's still listed
		dag, err := store.GetMetadata(ctx, dagName)
		require.NoError(t, err)
		assert.NotNil(t, dag)
		
		// Unsuspend it
		err = store.ToggleSuspend(ctx, dagName, false)
		require.NoError(t, err)
		assert.False(t, store.IsSuspended(ctx, dagName))
	})

	// Step 11: Test spec update with prefixed DAGs
	t.Run("update spec of prefixed DAG", func(t *testing.T) {
		dagName := "monitoring/health_check"
		newSpec := `name: health_check
description: Enhanced system health check
tags: [monitoring, critical, enhanced]
steps:
  - name: check_cpu
    command: bash check_cpu.sh
  - name: check_memory
    command: bash check_memory.sh
  - name: check_disk
    command: bash check_disk.sh`
		
		err := store.UpdateSpec(ctx, dagName, []byte(newSpec))
		require.NoError(t, err)
		
		// Verify the update
		dag, err := store.GetDetails(ctx, dagName)
		require.NoError(t, err)
		assert.Equal(t, "Enhanced system health check", dag.Description)
		assert.Len(t, dag.Steps, 3)
		assert.Contains(t, dag.Tags, "enhanced")
	})

	// Step 12: Test creating DAG with same name in different directories
	t.Run("same name in different directories", func(t *testing.T) {
		// Create config.yaml in multiple directories
		configs := []string{
			"config",
			"etl/config",
			"monitoring/config",
		}
		
		for _, name := range configs {
			spec := `name: config
description: Configuration for ` + filepath.Dir(name) + `
steps:
  - name: load_config
    command: echo "Loading config"`
			
			err := store.Create(ctx, name, []byte(spec))
			require.NoError(t, err)
		}
		
		// Verify all three exist independently
		for _, name := range configs {
			dag, err := store.GetMetadata(ctx, name)
			require.NoError(t, err)
			assert.Equal(t, name, dag.FileName())
			assert.Equal(t, "config", dag.Name)
		}
	})
}