package gitsync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
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
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	repoPath := filepath.Join(tempDir, "repo")
	err = os.MkdirAll(repoPath, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Initialize a real git repo for testing
	repo, err := git.PlainInit(repoPath, false)
	if err != nil {
		t.Fatal(err)
	}

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
	if !c.IsCloned() {
		t.Error("IsCloned() = false, want true")
	}

	// Test FileExists
	testFile := "test.yaml"
	fullPath := filepath.Join(repoPath, testFile)
	err = os.WriteFile(fullPath, []byte("test content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	if !c.FileExists(testFile) {
		t.Errorf("FileExists(%s) = false, want true", testFile)
	}

	// Test AddAndCommit
	hash, err := c.AddAndCommit(testFile, "initial commit")
	if err != nil {
		t.Fatalf("AddAndCommit failed: %v", err)
	}
	if hash == "" {
		t.Error("AddAndCommit returned empty hash")
	}

	// Test GetHeadCommit
	head, err := c.GetHeadCommit()
	if err != nil {
		t.Fatalf("GetHeadCommit failed: %v", err)
	}
	if head != hash {
		t.Errorf("GetHeadCommit() = %v, want %v", head, hash)
	}

	// Test ListFiles
	files, err := c.ListFiles([]string{".yaml"})
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}
	if len(files) != 1 || files[0] != testFile {
		t.Errorf("ListFiles() = %v, want [%s]", files, testFile)
	}
}
