package gitsync

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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
