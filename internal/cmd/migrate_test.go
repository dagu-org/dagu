package cmd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/persis/filedagrun"
	legacymodel "github.com/dagu-org/dagu/internal/persis/legacy/model"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrateHistoryCommand(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	dagRunsDir := filepath.Join(tempDir, "dag-runs")
	dagsDir := filepath.Join(tempDir, "dags")

	require.NoError(t, os.MkdirAll(dataDir, 0750))
	require.NoError(t, os.MkdirAll(dagRunsDir, 0750))
	require.NoError(t, os.MkdirAll(dagsDir, 0750))

	// Create legacy data
	legacyDagDir := filepath.Join(dataDir, "test-dag-abc123")
	require.NoError(t, os.MkdirAll(legacyDagDir, 0750))

	// Create legacy status
	legacyStatus := legacymodel.Status{
		RequestID:  "req123",
		Name:       "test-dag",
		Status:     core.Succeeded,
		StartedAt:  time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		FinishedAt: time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
		Nodes: []*legacymodel.Node{
			{
				Step:       core.Step{Name: "step1"},
				Status:     core.NodeSucceeded,
				StartedAt:  time.Now().Add(-50 * time.Minute).Format(time.RFC3339),
				FinishedAt: time.Now().Add(-40 * time.Minute).Format(time.RFC3339),
			},
		},
	}

	// Write legacy data file
	statusData, _ := json.Marshal(legacyStatus)
	datFile := filepath.Join(legacyDagDir, "test-dag.20240101.100000.req123.dat")
	require.NoError(t, os.WriteFile(datFile, statusData, 0600))

	// Create DAG file
	dagPath := filepath.Join(dagsDir, "test-dag.yaml")
	require.NoError(t, os.WriteFile(dagPath, []byte("name: test-dag\nsteps:\n  - name: step1\n    command: echo test"), 0600))

	// Create a test context
	cfg := &config.Config{
		Paths: config.PathsConfig{
			DataDir:    dataDir,
			DAGRunsDir: dagRunsDir,
			DAGsDir:    dagsDir,
		},
	}

	// Create stores
	dagRunStore := filedagrun.New(dagRunsDir)

	// Create command context
	cmd := &cobra.Command{}
	ctx := &Context{
		Context:     context.Background(),
		Command:     cmd,
		Config:      cfg,
		DAGRunStore: dagRunStore,
	}

	t.Run("SuccessfulMigration", func(t *testing.T) {
		err := runMigration(ctx)
		require.NoError(t, err)

		// Verify migration
		attempt, err := dagRunStore.FindAttempt(context.Background(), exec.NewDAGRunRef("test-dag", "req123"))
		require.NoError(t, err)
		require.NotNil(t, attempt)

		dagRunStatus, err := attempt.ReadStatus(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "req123", dagRunStatus.DAGRunID)
		assert.Equal(t, "test-dag", dagRunStatus.Name)
		assert.Equal(t, core.Succeeded, dagRunStatus.Status)

		// Verify legacy directory was moved
		_, err = os.Stat(legacyDagDir)
		assert.True(t, os.IsNotExist(err))

		// Verify archive exists
		entries, err := os.ReadDir(dataDir)
		require.NoError(t, err)

		archiveFound := false
		for _, entry := range entries {
			if entry.IsDir() && len(entry.Name()) > 17 && entry.Name()[:17] == "history_migrated_" {
				archiveFound = true
			}
		}
		assert.True(t, archiveFound)
	})
}

func TestMigrateCommand_NoLegacyData(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	dagRunsDir := filepath.Join(tempDir, "dag-runs")
	dagsDir := filepath.Join(tempDir, "dags")

	require.NoError(t, os.MkdirAll(dataDir, 0750))
	require.NoError(t, os.MkdirAll(dagRunsDir, 0750))
	require.NoError(t, os.MkdirAll(dagsDir, 0750))

	// Create a test context
	cfg := &config.Config{
		Paths: config.PathsConfig{
			DataDir:    dataDir,
			DAGRunsDir: dagRunsDir,
			DAGsDir:    dagsDir,
		},
	}

	// Create stores
	dagRunStore := filedagrun.New(dagRunsDir)

	// Create command context
	cmd := &cobra.Command{}
	ctx := &Context{
		Context:     context.Background(),
		Command:     cmd,
		Config:      cfg,
		DAGRunStore: dagRunStore,
	}

	// Run migration with no legacy data
	err := runMigration(ctx)
	require.NoError(t, err)

	// Should complete without errors
}

func TestCmdMigrate(t *testing.T) {
	cmd := Migrate()
	assert.NotNil(t, cmd)
	assert.Equal(t, "migrate", cmd.Use)
	assert.True(t, cmd.HasSubCommands())

	subcommands := map[string]bool{}
	for _, sub := range cmd.Commands() {
		subcommands[sub.Use] = true
	}
	assert.True(t, subcommands["history"], "history subcommand should exist")
	assert.True(t, subcommands["namespace"], "namespace subcommand should exist")
}

// --- Namespace migration tests ---

// newTestPaths creates a PathsConfig with temporary directories for namespace migration tests.
func newTestPaths(t *testing.T) config.PathsConfig {
	t.Helper()
	tempDir := t.TempDir()

	paths := config.PathsConfig{
		DataDir:         filepath.Join(tempDir, "data"),
		DAGsDir:         filepath.Join(tempDir, "dags"),
		DAGRunsDir:      filepath.Join(tempDir, "data", "dag-runs"),
		ProcDir:         filepath.Join(tempDir, "data", "proc"),
		QueueDir:        filepath.Join(tempDir, "data", "queue"),
		SuspendFlagsDir: filepath.Join(tempDir, "data", "suspend"),
		ConversationsDir: filepath.Join(tempDir, "data", "conversations"),
	}

	for _, dir := range []string{
		paths.DataDir, paths.DAGsDir, paths.DAGRunsDir,
		paths.ProcDir, paths.QueueDir, paths.SuspendFlagsDir,
		paths.ConversationsDir,
	} {
		require.NoError(t, os.MkdirAll(dir, 0750))
	}

	return paths
}

func TestNeedsNamespaceMigration(t *testing.T) {
	t.Run("ReturnsFalseWhenMarkerExists", func(t *testing.T) {
		paths := newTestPaths(t)
		// Write marker file
		markerPath := filepath.Join(paths.DataDir, namespaceMigratedMarker)
		require.NoError(t, os.WriteFile(markerPath, []byte("migrated\n"), 0600))

		needed, reason := needsNamespaceMigration(paths)
		assert.False(t, needed)
		assert.Empty(t, reason)
	})

	t.Run("ReturnsFalseWhenAlreadyScoped", func(t *testing.T) {
		tempDir := t.TempDir()
		paths := config.PathsConfig{
			DataDir:    filepath.Join(tempDir, "data"),
			DAGsDir:    filepath.Join(tempDir, "dags"),
			DAGRunsDir: filepath.Join(tempDir, "data", "0000", "dag-runs"),
			ProcDir:    filepath.Join(tempDir, "data", "0000", "proc"),
			QueueDir:   filepath.Join(tempDir, "data", "0000", "queue"),
		}
		for _, dir := range []string{paths.DataDir, paths.DAGsDir, paths.DAGRunsDir} {
			require.NoError(t, os.MkdirAll(dir, 0750))
		}

		needed, reason := needsNamespaceMigration(paths)
		assert.False(t, needed)
		assert.Empty(t, reason)
	})

	t.Run("ReturnsTrueWhenUnmigrated", func(t *testing.T) {
		paths := newTestPaths(t)
		// Create a YAML file at the root of DAGsDir to simulate unmigrated state
		require.NoError(t, os.WriteFile(filepath.Join(paths.DAGsDir, "my-dag.yaml"), []byte("name: my-dag"), 0600))

		needed, reason := needsNamespaceMigration(paths)
		assert.True(t, needed)
		assert.Contains(t, reason, "dagu migrate namespace")
	})

	t.Run("ReturnsFalseForFreshInstall", func(t *testing.T) {
		paths := newTestPaths(t)
		// Empty dirs — fresh install

		needed, reason := needsNamespaceMigration(paths)
		assert.False(t, needed)
		assert.Empty(t, reason)
	})
}

func TestRunNamespaceMigration(t *testing.T) {
	t.Run("MigratesDAGFiles", func(t *testing.T) {
		paths := newTestPaths(t)
		// Create YAML files at root of DAGsDir
		require.NoError(t, os.WriteFile(filepath.Join(paths.DAGsDir, "a.yaml"), []byte("name: a"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(paths.DAGsDir, "b.yml"), []byte("name: b"), 0600))

		result, err := runNamespaceMigration(paths, false)
		require.NoError(t, err)
		assert.Equal(t, 2, result.DAGFilesMoved)
		assert.False(t, result.AlreadyMigrated)
		assert.False(t, result.AlreadyScoped)

		// Verify files were moved
		assert.FileExists(t, filepath.Join(paths.DAGsDir, "0000", "a.yaml"))
		assert.FileExists(t, filepath.Join(paths.DAGsDir, "0000", "b.yml"))
		assert.NoFileExists(t, filepath.Join(paths.DAGsDir, "a.yaml"))
		assert.NoFileExists(t, filepath.Join(paths.DAGsDir, "b.yml"))

		// Verify marker was written
		assert.FileExists(t, filepath.Join(paths.DataDir, namespaceMigratedMarker))
	})

	t.Run("DryRunDoesNotMove", func(t *testing.T) {
		paths := newTestPaths(t)
		require.NoError(t, os.WriteFile(filepath.Join(paths.DAGsDir, "c.yaml"), []byte("name: c"), 0600))

		result, err := runNamespaceMigration(paths, true)
		require.NoError(t, err)
		assert.Equal(t, 1, result.DAGFilesMoved)

		// File should still be at root
		assert.FileExists(t, filepath.Join(paths.DAGsDir, "c.yaml"))
		// No marker written
		assert.NoFileExists(t, filepath.Join(paths.DataDir, namespaceMigratedMarker))
	})

	t.Run("IdempotentAfterMarker", func(t *testing.T) {
		paths := newTestPaths(t)
		// Write marker
		markerPath := filepath.Join(paths.DataDir, namespaceMigratedMarker)
		require.NoError(t, os.WriteFile(markerPath, []byte("migrated\n"), 0600))
		// Create a YAML file that would be migrated
		require.NoError(t, os.WriteFile(filepath.Join(paths.DAGsDir, "d.yaml"), []byte("name: d"), 0600))

		result, err := runNamespaceMigration(paths, false)
		require.NoError(t, err)
		assert.True(t, result.AlreadyMigrated)
		assert.Equal(t, 0, result.totalMigrated())

		// File should still be at root (not moved because marker existed)
		assert.FileExists(t, filepath.Join(paths.DAGsDir, "d.yaml"))
	})

	t.Run("MovesDirContents", func(t *testing.T) {
		paths := newTestPaths(t)
		// Create entries in dag-runs dir
		require.NoError(t, os.MkdirAll(filepath.Join(paths.DAGRunsDir, "my-dag"), 0750))
		require.NoError(t, os.WriteFile(filepath.Join(paths.DAGRunsDir, "my-dag", "run.json"), []byte("{}"), 0600))

		result, err := runNamespaceMigration(paths, false)
		require.NoError(t, err)
		assert.Equal(t, 1, result.DirEntriesMoved["dag-runs"])

		// Verify entries moved
		assert.DirExists(t, filepath.Join(paths.DataDir, "0000", "dag-runs", "my-dag"))
	})

	t.Run("TagsConversations", func(t *testing.T) {
		paths := newTestPaths(t)
		// Create a conversation file without namespace
		userDir := filepath.Join(paths.ConversationsDir, "user1")
		require.NoError(t, os.MkdirAll(userDir, 0750))
		conv := map[string]any{"id": "conv1", "title": "test"}
		data, _ := json.Marshal(conv)
		require.NoError(t, os.WriteFile(filepath.Join(userDir, "conv1.json"), data, 0600))

		result, err := runNamespaceMigration(paths, false)
		require.NoError(t, err)
		assert.Equal(t, 1, result.ConversationsTagged)

		// Verify namespace was set
		updated, err := os.ReadFile(filepath.Join(userDir, "conv1.json"))
		require.NoError(t, err)
		var parsed map[string]any
		require.NoError(t, json.Unmarshal(updated, &parsed))
		assert.Equal(t, "default", parsed["namespace"])
	})
}
