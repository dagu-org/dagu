package gitsync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/require"
)

func TestGitClient_NormalizeRepoURL(t *testing.T) {
	tests := []struct {
		name     string
		repo     string
		expected string
	}{
		{
			name:     "https url",
			repo:     "https://github.com/dagu-org/dagu.git",
			expected: "https://github.com/dagu-org/dagu.git",
		},
		{
			name:     "http url",
			repo:     "http://github.com/dagu-org/dagu.git",
			expected: "http://github.com/dagu-org/dagu.git",
		},
		{
			name:     "ssh url",
			repo:     "git@github.com:dagu-org/dagu.git",
			expected: "git@github.com:dagu-org/dagu.git",
		},
		{
			name:     "short format",
			repo:     "github.com/dagu-org/dagu",
			expected: "https://github.com/dagu-org/dagu.git",
		},
		{
			name:     "empty",
			repo:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Repository: tt.repo}
			c := NewGitClient(cfg, "")
			if got := c.normalizeRepoURL(); got != tt.expected {
				t.Errorf("normalizeRepoURL() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGitClient_LocalOps(t *testing.T) {
	// Setup temporary repo
	tempDir, err := os.MkdirTemp("", "gitsync-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	repoPath := filepath.Join(tempDir, "repo")
	require.NoError(t, os.MkdirAll(repoPath, 0755))

	// Initialize a real git repo for testing
	repo, err := git.PlainInit(repoPath, false)
	require.NoError(t, err)

	cfg := &Config{
		Enabled:    true,
		Repository: repoPath,
		Branch:     "main",
		Commit: CommitConfig{
			AuthorName:  "Test User",
			AuthorEmail: "test@example.com",
		},
	}
	c := NewGitClient(cfg, repoPath)
	c.repo = repo

	// Test IsCloned
	require.True(t, c.IsCloned())

	// Test FileExists
	testFile := "test.yaml"
	fullPath := filepath.Join(repoPath, testFile)
	require.NoError(t, os.WriteFile(fullPath, []byte("test content"), 0644))
	require.True(t, c.FileExists(testFile), "FileExists(%s) should be true", testFile)

	// Test AddAndCommit
	hash, err := c.AddAndCommit(testFile, "initial commit")
	require.NoError(t, err)
	require.NotEmpty(t, hash, "AddAndCommit should return non-empty hash")

	// Test GetHeadCommit
	head, err := c.GetHeadCommit()
	require.NoError(t, err)
	require.Equal(t, hash, head)

	// Test ListFiles
	files, err := c.ListFiles([]string{".yaml"})
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, testFile, files[0])
}

func TestGitClient_AddAndCommit_NoChanges(t *testing.T) {
	repoPath := t.TempDir()

	repo, err := git.PlainInit(repoPath, false)
	require.NoError(t, err)

	cfg := &Config{
		Enabled:    true,
		Repository: repoPath,
		Branch:     "main",
		Commit: CommitConfig{
			AuthorName:  "Test User",
			AuthorEmail: "test@example.com",
		},
	}
	c := NewGitClient(cfg, repoPath)
	c.repo = repo

	// Create and commit a file
	testFile := "dag.yaml"
	require.NoError(t, os.WriteFile(filepath.Join(repoPath, testFile), []byte("content"), 0644))
	firstHash, err := c.AddAndCommit(testFile, "first commit")
	require.NoError(t, err)
	require.NotEmpty(t, firstHash)

	// Re-write identical content and commit again — should return HEAD, not error
	require.NoError(t, os.WriteFile(filepath.Join(repoPath, testFile), []byte("content"), 0644))

	// Re-open the repo (as Publish does)
	c2 := NewGitClient(cfg, repoPath)
	require.NoError(t, c2.Open())

	secondHash, err := c2.AddAndCommit(testFile, "duplicate commit")
	require.NoError(t, err)
	require.Equal(t, firstHash, secondHash, "should return existing HEAD hash when content unchanged")
}

func TestGitClient_CommitStaged_NoChanges(t *testing.T) {
	repoPath := t.TempDir()

	repo, err := git.PlainInit(repoPath, false)
	require.NoError(t, err)

	cfg := &Config{
		Enabled:    true,
		Repository: repoPath,
		Branch:     "main",
		Commit: CommitConfig{
			AuthorName:  "Test User",
			AuthorEmail: "test@example.com",
		},
	}
	c := NewGitClient(cfg, repoPath)
	c.repo = repo

	// Create and commit a file
	require.NoError(t, os.WriteFile(filepath.Join(repoPath, "dag.yaml"), []byte("content"), 0644))
	firstHash, err := c.AddAndCommit("dag.yaml", "first commit")
	require.NoError(t, err)

	// CommitStaged with no staged changes should return HEAD
	c2 := NewGitClient(cfg, repoPath)
	require.NoError(t, c2.Open())

	hash, err := c2.CommitStaged("empty commit")
	require.NoError(t, err)
	require.Equal(t, firstHash, hash)
}
