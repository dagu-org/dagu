package cmd

import (
	"context"
	"encoding/json"
	"fmt"
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
	t.Parallel()

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

// --- Namespace migration tests ---

// newTestPaths creates a PathsConfig with temporary directories for namespace migration tests.
func newTestPaths(t *testing.T) config.PathsConfig {
	t.Helper()
	tempDir := t.TempDir()

	paths := config.PathsConfig{
		DataDir:          filepath.Join(tempDir, "data"),
		DAGsDir:          filepath.Join(tempDir, "dags"),
		DAGRunsDir:       filepath.Join(tempDir, "data", "dag-runs"),
		ProcDir:          filepath.Join(tempDir, "data", "proc"),
		QueueDir:         filepath.Join(tempDir, "data", "queue"),
		SuspendFlagsDir:  filepath.Join(tempDir, "data", "suspend"),
		ConversationsDir: filepath.Join(tempDir, "data", "conversations"),
		LogDir:           filepath.Join(tempDir, "logs"),
		AdminLogsDir:     filepath.Join(tempDir, "logs", "admin"),
		NamespacesDir:    filepath.Join(tempDir, "data", "namespaces"),
	}

	for _, dir := range []string{
		paths.DataDir, paths.DAGsDir, paths.DAGRunsDir,
		paths.ProcDir, paths.QueueDir, paths.SuspendFlagsDir,
		paths.ConversationsDir, paths.LogDir, paths.AdminLogsDir,
		paths.NamespacesDir,
	} {
		require.NoError(t, os.MkdirAll(dir, 0750))
	}

	return paths
}

func TestNeedsNamespaceMigration(t *testing.T) {
	t.Parallel()

	t.Run("ReturnsFalseWhenMarkerExists", func(t *testing.T) {
		t.Parallel()
		paths := newTestPaths(t)
		markerPath := filepath.Join(paths.DataDir, namespaceMigratedMarker)
		require.NoError(t, os.WriteFile(markerPath, []byte("migrated\n"), 0600))

		needed, reason := needsNamespaceMigration(paths)
		assert.False(t, needed)
		assert.Empty(t, reason)
	})

	t.Run("ReturnsFalseWhenAlreadyScoped", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
		paths := newTestPaths(t)
		require.NoError(t, os.WriteFile(filepath.Join(paths.DAGsDir, "my-dag.yaml"), []byte("name: my-dag"), 0600))

		needed, reason := needsNamespaceMigration(paths)
		assert.True(t, needed)
		assert.Contains(t, reason, "dagu migrate namespace")
	})

	t.Run("ReturnsFalseForFreshInstall", func(t *testing.T) {
		t.Parallel()
		paths := newTestPaths(t)

		needed, reason := needsNamespaceMigration(paths)
		assert.False(t, needed)
		assert.Empty(t, reason)
	})
}

func TestRunNamespaceMigration(t *testing.T) {
	t.Parallel()

	t.Run("MigratesDAGFiles", func(t *testing.T) {
		t.Parallel()
		paths := newTestPaths(t)
		require.NoError(t, os.WriteFile(filepath.Join(paths.DAGsDir, "a.yaml"), []byte("name: a"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(paths.DAGsDir, "b.yml"), []byte("name: b"), 0600))

		result, err := runNamespaceMigration(paths, false)
		require.NoError(t, err)
		assert.Equal(t, 2, result.DAGFilesMoved)
		assert.False(t, result.AlreadyMigrated)
		assert.False(t, result.AlreadyScoped)

		assert.FileExists(t, filepath.Join(paths.DAGsDir, "0000", "a.yaml"))
		assert.FileExists(t, filepath.Join(paths.DAGsDir, "0000", "b.yml"))
		assert.NoFileExists(t, filepath.Join(paths.DAGsDir, "a.yaml"))
		assert.NoFileExists(t, filepath.Join(paths.DAGsDir, "b.yml"))
		assert.FileExists(t, filepath.Join(paths.DataDir, namespaceMigratedMarker))
	})

	t.Run("DryRunDoesNotMove", func(t *testing.T) {
		t.Parallel()
		paths := newTestPaths(t)
		require.NoError(t, os.WriteFile(filepath.Join(paths.DAGsDir, "c.yaml"), []byte("name: c"), 0600))

		result, err := runNamespaceMigration(paths, true)
		require.NoError(t, err)
		assert.Equal(t, 1, result.DAGFilesMoved)

		assert.FileExists(t, filepath.Join(paths.DAGsDir, "c.yaml"))
		assert.NoFileExists(t, filepath.Join(paths.DataDir, namespaceMigratedMarker))
	})

	t.Run("IdempotentAfterMarker", func(t *testing.T) {
		t.Parallel()
		paths := newTestPaths(t)
		markerPath := filepath.Join(paths.DataDir, namespaceMigratedMarker)
		require.NoError(t, os.WriteFile(markerPath, []byte("migrated\n"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(paths.DAGsDir, "d.yaml"), []byte("name: d"), 0600))

		result, err := runNamespaceMigration(paths, false)
		require.NoError(t, err)
		assert.True(t, result.AlreadyMigrated)
		assert.Equal(t, 0, result.totalMigrated())
		assert.FileExists(t, filepath.Join(paths.DAGsDir, "d.yaml"))
	})

	t.Run("MovesDirContents", func(t *testing.T) {
		t.Parallel()
		paths := newTestPaths(t)
		require.NoError(t, os.MkdirAll(filepath.Join(paths.DAGRunsDir, "my-dag"), 0750))
		require.NoError(t, os.WriteFile(filepath.Join(paths.DAGRunsDir, "my-dag", "run.json"), []byte("{}"), 0600))

		result, err := runNamespaceMigration(paths, false)
		require.NoError(t, err)
		assert.Equal(t, 1, result.DirEntriesMoved["dag-runs"])
		assert.DirExists(t, filepath.Join(paths.DataDir, "0000", "dag-runs", "my-dag"))
	})

	t.Run("TagsConversations", func(t *testing.T) {
		t.Parallel()
		paths := newTestPaths(t)
		userDir := filepath.Join(paths.ConversationsDir, "user1")
		require.NoError(t, os.MkdirAll(userDir, 0750))
		conv := map[string]any{"id": "conv1", "title": "test"}
		data, _ := json.Marshal(conv)
		require.NoError(t, os.WriteFile(filepath.Join(userDir, "conv1.json"), data, 0600))

		result, err := runNamespaceMigration(paths, false)
		require.NoError(t, err)
		assert.Equal(t, 1, result.ConversationsTagged)

		updated, err := os.ReadFile(filepath.Join(userDir, "conv1.json"))
		require.NoError(t, err)
		var parsed map[string]any
		require.NoError(t, json.Unmarshal(updated, &parsed))
		assert.Equal(t, "default", parsed["namespace"])
	})
}

func TestMigrateLogDir(t *testing.T) {
	t.Parallel()

	t.Run("MovesEntries", func(t *testing.T) {
		t.Parallel()
		paths := newTestPaths(t)

		require.NoError(t, os.MkdirAll(filepath.Join(paths.LogDir, "mydag"), 0750))
		require.NoError(t, os.MkdirAll(filepath.Join(paths.LogDir, "otherdag"), 0750))
		require.NoError(t, os.WriteFile(filepath.Join(paths.NamespacesDir, "0000.json"), []byte("{}"), 0600))

		n, err := migrateLogDir(paths.LogDir, paths.AdminLogsDir, paths.NamespacesDir, "0000", false)
		require.NoError(t, err)
		assert.Equal(t, 2, n)

		assert.DirExists(t, filepath.Join(paths.LogDir, "0000", "mydag"))
		assert.DirExists(t, filepath.Join(paths.LogDir, "0000", "otherdag"))
		assert.NoDirExists(t, filepath.Join(paths.LogDir, "mydag"))
		assert.NoDirExists(t, filepath.Join(paths.LogDir, "otherdag"))
		assert.DirExists(t, paths.AdminLogsDir)
	})

	t.Run("SkipsNamespaceDirs", func(t *testing.T) {
		t.Parallel()
		paths := newTestPaths(t)

		require.NoError(t, os.MkdirAll(filepath.Join(paths.LogDir, "a1b2"), 0750))
		require.NoError(t, os.WriteFile(filepath.Join(paths.NamespacesDir, "a1b2.json"), []byte("{}"), 0600))

		n, err := migrateLogDir(paths.LogDir, paths.AdminLogsDir, paths.NamespacesDir, "0000", false)
		require.NoError(t, err)
		assert.Equal(t, 0, n)
		assert.DirExists(t, filepath.Join(paths.LogDir, "a1b2"))
	})
}

func TestFixLogPathsInStatusFiles(t *testing.T) {
	t.Parallel()

	t.Run("FixesOldPaths", func(t *testing.T) {
		t.Parallel()
		paths := newTestPaths(t)

		dagRunDir := filepath.Join(paths.DataDir, "0000", "dag-runs", "mydag", "run1")
		require.NoError(t, os.MkdirAll(dagRunDir, 0750))

		oldPath := paths.LogDir + "/mydag/run1.log"
		expectedPath := filepath.Join(paths.LogDir, "0000") + "/mydag/run1.log"
		content := fmt.Sprintf(`{"Log":"%s","Name":"mydag"}`, oldPath)
		require.NoError(t, os.WriteFile(filepath.Join(dagRunDir, "status.jsonl"), []byte(content), 0600))

		n, err := fixLogPathsInStatusFiles(paths.DataDir, paths.LogDir, "0000", false)
		require.NoError(t, err)
		assert.Equal(t, 1, n)

		updated, err := os.ReadFile(filepath.Join(dagRunDir, "status.jsonl"))
		require.NoError(t, err)
		assert.Contains(t, string(updated), expectedPath)
		assert.NotContains(t, string(updated), oldPath)
	})

	t.Run("ThreeStepSafety", func(t *testing.T) {
		t.Parallel()
		paths := newTestPaths(t)

		dagRunDir := filepath.Join(paths.DataDir, "0000", "dag-runs", "mydag", "run1")
		require.NoError(t, os.MkdirAll(dagRunDir, 0750))

		oldPath := paths.LogDir + "/mydag/run1.log"
		scopedPath := filepath.Join(paths.LogDir, "0000") + "/mydag/run2.log"
		content := fmt.Sprintf("{\"Log\":\"%s\"}\n{\"Log\":\"%s\"}", oldPath, scopedPath)
		require.NoError(t, os.WriteFile(filepath.Join(dagRunDir, "status.jsonl"), []byte(content), 0600))

		n, err := fixLogPathsInStatusFiles(paths.DataDir, paths.LogDir, "0000", false)
		require.NoError(t, err)
		assert.Equal(t, 1, n)

		updated, err := os.ReadFile(filepath.Join(dagRunDir, "status.jsonl"))
		require.NoError(t, err)
		s := string(updated)
		expectedFixed := filepath.Join(paths.LogDir, "0000") + "/mydag/run1.log"
		assert.Contains(t, s, expectedFixed)
		assert.Contains(t, s, scopedPath)
		assert.NotContains(t, s, filepath.Join(paths.LogDir, "0000", "0000"))
	})
}

func TestRunNamespaceMigration_MigratesLogs(t *testing.T) {
	t.Parallel()

	paths := newTestPaths(t)

	require.NoError(t, os.MkdirAll(filepath.Join(paths.LogDir, "mydag"), 0750))
	require.NoError(t, os.WriteFile(filepath.Join(paths.NamespacesDir, "0000.json"), []byte("{}"), 0600))

	dagRunDir := filepath.Join(paths.DAGRunsDir, "mydag", "run1")
	require.NoError(t, os.MkdirAll(dagRunDir, 0750))
	oldPath := paths.LogDir + "/mydag/run1.log"
	content := fmt.Sprintf(`{"Log":"%s","Name":"mydag"}`, oldPath)
	require.NoError(t, os.WriteFile(filepath.Join(dagRunDir, "status.jsonl"), []byte(content), 0600))

	result, err := runNamespaceMigration(paths, false)
	require.NoError(t, err)

	assert.Greater(t, result.LogEntriesMoved, 0)
	assert.Greater(t, result.StatusFilesFixed, 0)

	assert.DirExists(t, filepath.Join(paths.LogDir, "0000", "mydag"))
	assert.NoDirExists(t, filepath.Join(paths.LogDir, "mydag"))

	updated, err := os.ReadFile(filepath.Join(paths.DataDir, "0000", "dag-runs", "mydag", "run1", "status.jsonl"))
	require.NoError(t, err)
	expectedPath := filepath.Join(paths.LogDir, "0000") + "/mydag/run1.log"
	assert.Contains(t, string(updated), expectedPath)
}

func TestHasUnmigratedData_ChecksLogDir(t *testing.T) {
	t.Parallel()

	t.Run("DetectsNonNamespaceLogDir", func(t *testing.T) {
		t.Parallel()
		paths := newTestPaths(t)
		require.NoError(t, os.MkdirAll(filepath.Join(paths.LogDir, "mydag"), 0750))

		assert.True(t, hasUnmigratedData(paths))
	})

	t.Run("IgnoresAdminAndNamespaceDirs", func(t *testing.T) {
		t.Parallel()
		paths := newTestPaths(t)
		require.NoError(t, os.MkdirAll(filepath.Join(paths.LogDir, "0000"), 0750))

		assert.False(t, hasUnmigratedData(paths))
	})
}

func TestIsHexString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  bool
	}{
		{"0000", true},
		{"a1b2", true},
		{"deadbeef", true},
		{"my-dag", false},
		{"", false},
		{"ABCD", false},
		{"0000/", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, isHexString(tt.input), "isHexString(%q)", tt.input)
	}
}
