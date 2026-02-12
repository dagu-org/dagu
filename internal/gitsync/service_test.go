package gitsync

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_GetStatus(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gitsync-svc-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	dagsDir := filepath.Join(tempDir, "dags")
	dataDir := filepath.Join(tempDir, "data")
	require.NoError(t, os.MkdirAll(dagsDir, 0755))
	require.NoError(t, os.MkdirAll(dataDir, 0755))

	cfg := &Config{
		Enabled:    true,
		Repository: "host.com/org/repo",
		Branch:     "main",
	}

	svc := NewService(cfg, dagsDir, dataDir)
	status, err := svc.GetStatus(context.Background())
	require.NoError(t, err)

	require.True(t, status.Enabled)
	require.Equal(t, cfg.Repository, status.Repository)
	require.Equal(t, cfg.Branch, status.Branch)
}

func TestService_PathHelpers(t *testing.T) {
	s := &serviceImpl{
		dagsDir: "/dags",
		cfg: &Config{
			Path: "subdir",
		},
	}

	// Test filePathToDAGID
	dagID := s.filePathToDAGID("subdir/my_dag.yaml")
	require.Equal(t, "my_dag", dagID)

	// Test dagIDToFilePath
	dagPath := s.dagIDToFilePath("my_dag")
	require.Equal(t, "/dags/my_dag.yaml", dagPath)

	// Test dagIDToRepoPath
	repoPath := s.dagIDToRepoPath("my_dag")
	require.Equal(t, "subdir/my_dag.yaml", repoPath)
}

func TestIsMemoryFile(t *testing.T) {
	t.Parallel()

	assert.True(t, isMemoryFile("memory/MEMORY"))
	assert.True(t, isMemoryFile("memory/dags/my-dag/MEMORY"))
	assert.False(t, isMemoryFile("my-dag"))
	assert.False(t, isMemoryFile("memoryfile"))
}

func TestFileExtensionForID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, ".md", fileExtensionForID("memory/MEMORY"))
	assert.Equal(t, ".md", fileExtensionForID("memory/dags/my-dag/MEMORY"))
	assert.Equal(t, ".yaml", fileExtensionForID("my-dag"))
	assert.Equal(t, ".yaml", fileExtensionForID("subdir/my-dag"))
}

func TestDagIDToFilePath_MemoryFiles(t *testing.T) {
	s := &serviceImpl{
		dagsDir: "/dags",
		cfg:     &Config{},
	}

	// Regular DAG
	assert.Equal(t, "/dags/my-dag.yaml", s.dagIDToFilePath("my-dag"))

	// Memory file
	assert.Equal(t,
		filepath.Join("/dags", "memory", "MEMORY.md"),
		s.dagIDToFilePath("memory/MEMORY"),
	)

	// DAG-specific memory
	assert.Equal(t,
		filepath.Join("/dags", "memory", "dags", "my-dag", "MEMORY.md"),
		s.dagIDToFilePath("memory/dags/my-dag/MEMORY"),
	)
}

func TestDagIDToRepoPath_MemoryFiles(t *testing.T) {
	s := &serviceImpl{
		dagsDir: "/dags",
		cfg:     &Config{Path: "subdir"},
	}

	// Regular DAG
	assert.Equal(t, "subdir/my-dag.yaml", s.dagIDToRepoPath("my-dag"))

	// Memory file
	assert.Equal(t,
		filepath.Join("subdir", "memory", "MEMORY.md"),
		s.dagIDToRepoPath("memory/MEMORY"),
	)
}

func TestScanMemoryFiles(t *testing.T) {
	tempDir := t.TempDir()
	s := &serviceImpl{
		dagsDir: tempDir,
		cfg:     &Config{},
	}

	// Create memory directory with files
	memDir := filepath.Join(tempDir, "memory")
	require.NoError(t, os.MkdirAll(memDir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(memDir, "MEMORY.md"), []byte("global memory"), 0600))

	dagMemDir := filepath.Join(memDir, "dags", "my-dag")
	require.NoError(t, os.MkdirAll(dagMemDir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(dagMemDir, "MEMORY.md"), []byte("dag memory"), 0600))

	state := &State{DAGs: make(map[string]*DAGState)}
	s.scanMemoryFiles(state)

	// Should find global memory
	globalID := filepath.Join("memory", "MEMORY")
	assert.Contains(t, state.DAGs, globalID)
	assert.Equal(t, StatusUntracked, state.DAGs[globalID].Status)

	// Should find per-DAG memory
	dagID := filepath.Join("memory", "dags", "my-dag", "MEMORY")
	assert.Contains(t, state.DAGs, dagID)
	assert.Equal(t, StatusUntracked, state.DAGs[dagID].Status)
}

func TestScanLocalDAGs_IgnoresNonMemoryMd(t *testing.T) {
	tempDir := t.TempDir()

	// Create a non-memory .md file at root (e.g., README.md)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "README.md"), []byte("# readme"), 0600))

	// Create a regular .yaml DAG
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "my-dag.yaml"), []byte("steps: []"), 0600))

	s := &serviceImpl{
		dagsDir: tempDir,
		cfg:     &Config{},
	}

	state := &State{DAGs: make(map[string]*DAGState)}
	err := s.scanLocalDAGs(state)
	require.NoError(t, err)

	// Should find the yaml DAG
	assert.Contains(t, state.DAGs, "my-dag")

	// Should NOT find README.md (it's not a yaml DAG or memory file at root)
	assert.NotContains(t, state.DAGs, "README")
}
