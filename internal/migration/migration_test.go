package migration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/persistence/filedagrun"
	legacyModel "github.com/dagu-org/dagu/internal/persistence/legacy/model"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDAGStore implements models.DAGStore for testing
type mockDAGStore struct {
	dags map[string]*digraph.DAG
}

func (m *mockDAGStore) GetDetails(ctx context.Context, path string, opts ...digraph.LoadOption) (*digraph.DAG, error) {
	if dag, ok := m.dags[path]; ok {
		return dag, nil
	}
	return nil, os.ErrNotExist
}

func (m *mockDAGStore) List(ctx context.Context, params models.ListDAGsOptions) (models.PaginatedResult[*digraph.DAG], []string, error) {
	var dags []*digraph.DAG
	for _, dag := range m.dags {
		dags = append(dags, dag)
	}
	return models.PaginatedResult[*digraph.DAG]{
		Items:      dags,
		TotalCount: len(dags),
	}, nil, nil
}

func (m *mockDAGStore) FindByName(ctx context.Context, name string) (*digraph.DAG, error) {
	for _, dag := range m.dags {
		if dag.Name == name {
			return dag, nil
		}
	}
	return nil, os.ErrNotExist
}

func (m *mockDAGStore) Create(ctx context.Context, name string, spec []byte) error {
	return nil
}

func (m *mockDAGStore) Delete(ctx context.Context, name string) error {
	return nil
}

func (m *mockDAGStore) Update(ctx context.Context, name string, spec []byte) error {
	return nil
}

func (m *mockDAGStore) Rename(ctx context.Context, oldName, newName string) error {
	return nil
}

func (m *mockDAGStore) GetSpec(ctx context.Context, name string) (string, error) {
	return "", nil
}

func (m *mockDAGStore) IsSuspended(ctx context.Context, name string) bool {
	return false
}

func (m *mockDAGStore) ToggleSuspend(ctx context.Context, name string, suspend bool) error {
	return nil
}

func (m *mockDAGStore) GetMetadata(ctx context.Context, name string) (*digraph.DAG, error) {
	return nil, nil
}


func (m *mockDAGStore) Grep(ctx context.Context, pattern string) ([]*models.GrepDAGsResult, []string, error) {
	return nil, nil, nil
}

func (m *mockDAGStore) UpdateSpec(ctx context.Context, fileName string, spec []byte) error {
	return nil
}

func (m *mockDAGStore) LoadSpec(ctx context.Context, spec []byte, opts ...digraph.LoadOption) (*digraph.DAG, error) {
	return nil, nil
}

func (m *mockDAGStore) TagList(ctx context.Context) ([]string, []string, error) {
	return nil, nil, nil
}

func TestExtractDAGName(t *testing.T) {
	tests := []struct {
		name     string
		dirName  string
		expected string
	}{
		{
			name:     "directory with hash",
			dirName:  "my-dag-a1b2c3d4",
			expected: "my-dag",
		},
		{
			name:     "directory with longer hash",
			dirName:  "test-workflow-deadbeef1234",
			expected: "test-workflow",
		},
		{
			name:     "directory without hash",
			dirName:  "simple-dag",
			expected: "simple-dag",
		},
		{
			name:     "directory with multiple hyphens",
			dirName:  "my-complex-dag-name-abc123",
			expected: "my-complex-dag-name",
		},
		{
			name:     "directory with non-hex suffix",
			dirName:  "dag-with-suffix-xyz",
			expected: "dag-with-suffix-xyz",
		},
	}

	m := &HistoryMigrator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.extractDAGName(tt.dirName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReadLegacyStatusFile(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()
	
	// Create a test status file with multiple lines
	testFile := filepath.Join(tempDir, "test.dat")
	status1 := legacyModel.Status{
		RequestID: "req123",
		Name:      "test-dag",
		Status:    scheduler.StatusRunning,
		StartedAt: "2024-01-01T10:00:00Z",
	}
	status2 := legacyModel.Status{
		RequestID: "req123",
		Name:      "test-dag",
		Status:    scheduler.StatusSuccess,
		StartedAt: "2024-01-01T10:00:00Z",
		FinishedAt: "2024-01-01T10:05:00Z",
	}

	// Write multiple status lines
	f, err := os.Create(testFile)
	require.NoError(t, err)
	
	data1, _ := json.Marshal(status1)
	data2, _ := json.Marshal(status2)
	f.WriteString(string(data1) + "\n")
	f.WriteString(string(data2) + "\n")
	f.Close()

	// Test reading the file
	m := &HistoryMigrator{}
	statusFile, err := m.readLegacyStatusFile(testFile)
	require.NoError(t, err)
	require.NotNil(t, statusFile)
	
	// Should get the last status (finished one)
	assert.Equal(t, "req123", statusFile.Status.RequestID)
	assert.Equal(t, scheduler.StatusSuccess, statusFile.Status.Status)
	assert.Equal(t, "2024-01-01T10:05:00Z", statusFile.Status.FinishedAt)
}

func TestNeedsMigration(t *testing.T) {
	ctx := context.Background()
	
	t.Run("no history directory", func(t *testing.T) {
		tempDir := t.TempDir()
		m := &HistoryMigrator{dataDir: tempDir}
		
		needsMigration, err := m.NeedsMigration(ctx)
		assert.NoError(t, err)
		assert.False(t, needsMigration)
	})

	t.Run("history directory with dat files", func(t *testing.T) {
		tempDir := t.TempDir()
		
		// Create legacy structure
		dagDir := filepath.Join(tempDir, "my-dag-abc123")
		require.NoError(t, os.MkdirAll(dagDir, 0755))
		
		datFile := filepath.Join(dagDir, "my-dag.20240101.100000.req123.dat")
		require.NoError(t, os.WriteFile(datFile, []byte(`{"RequestId":"req123"}`), 0644))
		
		m := &HistoryMigrator{dataDir: tempDir}
		needsMigration, err := m.NeedsMigration(ctx)
		assert.NoError(t, err)
		assert.True(t, needsMigration)
	})

	t.Run("history directory without dat files", func(t *testing.T) {
		tempDir := t.TempDir()
		
		// Create directory without .dat files
		dagDir := filepath.Join(tempDir, "my-dag-abc123")
		require.NoError(t, os.MkdirAll(dagDir, 0755))
		
		otherFile := filepath.Join(dagDir, "other.txt")
		require.NoError(t, os.WriteFile(otherFile, []byte("test"), 0644))
		
		m := &HistoryMigrator{dataDir: tempDir}
		needsMigration, err := m.NeedsMigration(ctx)
		assert.NoError(t, err)
		assert.False(t, needsMigration)
	})
}

func TestConvertStatus(t *testing.T) {
	legacy := &legacyModel.Status{
		RequestID:  "req123",
		Name:       "test-dag",
		Status:     scheduler.StatusSuccess,
		PID:        legacyModel.PID(12345),
		Log:        "test log",
		StartedAt:  "2024-01-01T10:00:00Z",
		FinishedAt: "2024-01-01T10:05:00Z",
		Params:     "param1=value1",
		ParamsList: []string{"param1", "value1"},
		Nodes: []*legacyModel.Node{
			{
				Step: digraph.Step{
					Name: "step1",
				},
				Status:     scheduler.NodeStatusSuccess,
				StartedAt:  "2024-01-01T10:01:00Z",
				FinishedAt: "2024-01-01T10:02:00Z",
				Log:        "step log",
			},
		},
		OnSuccess: &legacyModel.Node{
			Step: digraph.Step{
				Name: "on_success",
			},
			Status: scheduler.NodeStatusSuccess,
		},
	}

	dag := &digraph.DAG{
		Name: "test-dag",
		Preconditions: []*digraph.Condition{
			{Condition: "test condition"},
		},
	}

	m := &HistoryMigrator{}
	result := m.convertStatus(legacy, dag)

	assert.Equal(t, "test-dag", result.Name)
	assert.Equal(t, "req123", result.DAGRunID)
	assert.Equal(t, scheduler.StatusSuccess, result.Status)
	assert.Equal(t, models.PID(12345), result.PID)
	assert.Equal(t, "test log", result.Log)
	assert.Equal(t, "param1=value1", result.Params)
	assert.Equal(t, []string{"param1", "value1"}, result.ParamsList)
	assert.NotEmpty(t, result.StartedAt)
	assert.NotEmpty(t, result.FinishedAt)
	assert.Len(t, result.Nodes, 1)
	assert.NotNil(t, result.OnSuccess)
	assert.Equal(t, 1, len(result.Preconditions))
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		name      string
		timeStr   string
		shouldErr bool
	}{
		{
			name:      "RFC3339 format",
			timeStr:   "2024-01-01T10:00:00Z",
			shouldErr: false,
		},
		{
			name:      "RFC3339 with timezone",
			timeStr:   "2024-01-01T10:00:00+09:00",
			shouldErr: false,
		},
		{
			name:      "space separated format",
			timeStr:   "2024-01-01 10:00:00",
			shouldErr: false,
		},
		{
			name:      "empty string",
			timeStr:   "",
			shouldErr: true,
		},
		{
			name:      "dash only",
			timeStr:   "-",
			shouldErr: true,
		},
		{
			name:      "invalid format",
			timeStr:   "not a date",
			shouldErr: true,
		},
	}

	m := &HistoryMigrator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := m.parseTime(tt.timeStr)
			if tt.shouldErr {
				assert.Error(t, err)
				assert.True(t, result.IsZero())
			} else {
				assert.NoError(t, err)
				assert.False(t, result.IsZero())
			}
		})
	}
}

func TestMoveLegacyData(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	// Create legacy directories with .dat files
	legacyDir1 := filepath.Join(tempDir, "dag1-abc123")
	require.NoError(t, os.MkdirAll(legacyDir1, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir1, "test.dat"), []byte("data"), 0644))

	legacyDir2 := filepath.Join(tempDir, "dag2-def456")
	require.NoError(t, os.MkdirAll(legacyDir2, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir2, "test.dat"), []byte("data"), 0644))

	// Create a non-legacy directory
	otherDir := filepath.Join(tempDir, "other-dir")
	require.NoError(t, os.MkdirAll(otherDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(otherDir, "test.txt"), []byte("data"), 0644))

	m := &HistoryMigrator{dataDir: tempDir}
	err := m.MoveLegacyData(ctx)
	require.NoError(t, err)

	// Check that legacy directories were moved
	_, err = os.Stat(legacyDir1)
	assert.True(t, os.IsNotExist(err), "legacy directory 1 should be moved")

	_, err = os.Stat(legacyDir2)
	assert.True(t, os.IsNotExist(err), "legacy directory 2 should be moved")

	// Check that non-legacy directory still exists
	_, err = os.Stat(otherDir)
	assert.NoError(t, err, "non-legacy directory should still exist")

	// Check that archive directory was created
	entries, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	
	archiveFound := false
	for _, entry := range entries {
		if entry.IsDir() && len(entry.Name()) > 17 && entry.Name()[:17] == "history_migrated_" {
			archiveFound = true
			
			// Check archive contents
			archiveDir := filepath.Join(tempDir, entry.Name())
			archiveEntries, err := os.ReadDir(archiveDir)
			require.NoError(t, err)
			assert.Len(t, archiveEntries, 2, "should have 2 moved directories")
		}
	}
	assert.True(t, archiveFound, "archive directory should be created")
}

func TestLoadDAGForMigration(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	// Create test DAG files
	dag1Path := filepath.Join(tempDir, "test-dag.yaml")
	dag2Path := filepath.Join(tempDir, "test-dag-v2.yml")

	dag1 := &digraph.DAG{Name: "test-dag", Location: dag1Path}
	dag2 := &digraph.DAG{Name: "test-dag-v2", Location: dag2Path}

	mockStore := &mockDAGStore{
		dags: map[string]*digraph.DAG{
			dag1Path: dag1,
			dag2Path: dag2,
		},
	}

	m := &HistoryMigrator{
		dagStore: mockStore,
		dagsDir:  tempDir,
	}

	// Create the actual files
	require.NoError(t, os.WriteFile(dag1Path, []byte("test"), 0644))
	require.NoError(t, os.WriteFile(dag2Path, []byte("test"), 0644))

	t.Run("find by status name", func(t *testing.T) {
		result, err := m.loadDAGForMigration(ctx, "test-dag", "other-name")
		require.NoError(t, err)
		assert.Equal(t, "test-dag", result.Name)
	})

	t.Run("find by directory name", func(t *testing.T) {
		result, err := m.loadDAGForMigration(ctx, "not-found", "test-dag-v2")
		require.NoError(t, err)
		assert.Equal(t, "test-dag-v2", result.Name)
	})

	t.Run("not found - create minimal", func(t *testing.T) {
		result, err := m.loadDAGForMigration(ctx, "not-found", "also-not-found")
		require.NoError(t, err)
		assert.Equal(t, "not-found", result.Name)
	})
}

func TestFullMigration(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	
	// Set up directories - use subdirectories to avoid counting them as legacy dirs
	dataDir := filepath.Join(tempDir, "data")
	dagRunsDir := filepath.Join(tempDir, "runs")
	dagsDir := filepath.Join(tempDir, "dags")
	
	require.NoError(t, os.MkdirAll(dataDir, 0755))
	require.NoError(t, os.MkdirAll(dagRunsDir, 0755))
	require.NoError(t, os.MkdirAll(dagsDir, 0755))

	// Create legacy data
	legacyDagDir := filepath.Join(dataDir, "test-dag-abc123")
	require.NoError(t, os.MkdirAll(legacyDagDir, 0755))

	// Create legacy status
	legacyStatus := legacyModel.Status{
		RequestID:  "req123",
		Name:       "test-dag",
		Status:     scheduler.StatusSuccess,
		StartedAt:  time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		FinishedAt: time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
		Nodes: []*legacyModel.Node{
			{
				Step: digraph.Step{Name: "step1"},
				Status: scheduler.NodeStatusSuccess,
			},
		},
	}

	// Write legacy data file
	statusData, _ := json.Marshal(legacyStatus)
	datFile := filepath.Join(legacyDagDir, "test-dag.20240101.100000.req123.dat")
	require.NoError(t, os.WriteFile(datFile, statusData, 0644))

	// Create DAG file
	dagPath := filepath.Join(dagsDir, "test-dag.yaml")
	testDAG := &digraph.DAG{
		Name: "test-dag",
		Location: dagPath,
	}
	require.NoError(t, os.WriteFile(dagPath, []byte("name: test-dag"), 0644))

	// Set up stores
	dagRunStore := filedagrun.New(dagRunsDir)
	dagStore := &mockDAGStore{
		dags: map[string]*digraph.DAG{
			dagPath: testDAG,
		},
	}

	// Create migrator
	migrator := NewHistoryMigrator(dagRunStore, dagStore, dataDir, dagsDir)

	// Check migration is needed
	needsMigration, err := migrator.NeedsMigration(ctx)
	require.NoError(t, err)
	assert.True(t, needsMigration)

	// Run migration
	result, err := migrator.Migrate(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalDAGs)
	assert.Equal(t, 1, result.TotalRuns)
	assert.Equal(t, 1, result.MigratedRuns)
	assert.Equal(t, 0, result.FailedRuns)

	// Verify migration
	attempt, err := dagRunStore.FindAttempt(ctx, digraph.NewDAGRunRef("test-dag", "req123"))
	require.NoError(t, err)
	require.NotNil(t, attempt)

	status, err := attempt.ReadStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, "req123", status.DAGRunID)
	assert.Equal(t, "test-dag", status.Name)
	assert.Equal(t, scheduler.StatusSuccess, status.Status)
	assert.Len(t, status.Nodes, 1)

	// Move legacy data
	err = migrator.MoveLegacyData(ctx)
	require.NoError(t, err)

	// Verify legacy directory was moved
	_, err = os.Stat(legacyDagDir)
	assert.True(t, os.IsNotExist(err))
}