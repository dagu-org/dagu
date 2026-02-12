package filememory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
)

// Verify Store implements agent.MemoryStore at compile time.
var _ agent.MemoryStore = (*Store)(nil)

const (
	agentMemoryDir  = "agent-memory"
	dagSubDir       = "dags"
	memoryFileName  = "MEMORY.md"
	maxLines        = 200
	dirPermissions  = 0750
	filePermissions = 0600
)

// Store implements a file-based memory store for the agent.
// Memory files are stored under {dagsDir}/agent-memory/.
// Thread-safe through internal locking.
type Store struct {
	baseDir string // {dagsDir}/agent-memory
	mu      sync.RWMutex
}

// New creates a new file-based agent memory store.
// The dagsDir is the DAGs directory (e.g., ~/.config/dagu/dags).
// The memory files will be stored under {dagsDir}/agent-memory/.
func New(dagsDir string) (*Store, error) {
	if dagsDir == "" {
		return nil, errors.New("filememory: dagsDir cannot be empty")
	}

	baseDir := filepath.Join(dagsDir, agentMemoryDir)
	if err := os.MkdirAll(baseDir, dirPermissions); err != nil {
		return nil, fmt.Errorf("filememory: failed to create directory %s: %w", baseDir, err)
	}

	return &Store{baseDir: baseDir}, nil
}

// LoadGlobalMemory reads the global MEMORY.md, truncated to maxLines.
// Returns empty string if the file does not exist.
func (s *Store) LoadGlobalMemory(_ context.Context) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.readMemoryFile(s.globalMemoryPath())
}

// LoadDAGMemory reads the MEMORY.md for a specific DAG, truncated to maxLines.
// Returns empty string if the file does not exist.
func (s *Store) LoadDAGMemory(_ context.Context, dagName string) (string, error) {
	if err := s.validateDAGName(dagName); err != nil {
		return "", err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.readMemoryFile(s.dagMemoryPath(dagName))
}

// SaveGlobalMemory writes content to the global MEMORY.md atomically.
func (s *Store) SaveGlobalMemory(_ context.Context, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return fileutil.WriteFileAtomic(s.globalMemoryPath(), []byte(content), filePermissions)
}

// SaveDAGMemory writes content to a DAG-specific MEMORY.md atomically.
func (s *Store) SaveDAGMemory(_ context.Context, dagName string, content string) error {
	if err := s.validateDAGName(dagName); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	dagDir := filepath.Join(s.baseDir, dagSubDir, dagName)
	if err := os.MkdirAll(dagDir, dirPermissions); err != nil {
		return fmt.Errorf("filememory: failed to create DAG memory directory: %w", err)
	}

	return fileutil.WriteFileAtomic(s.dagMemoryPath(dagName), []byte(content), filePermissions)
}

// MemoryDir returns the root memory directory path.
func (s *Store) MemoryDir() string {
	return s.baseDir
}

// readMemoryFile reads a memory file and truncates it to maxLines.
// Returns empty string if the file does not exist.
func (s *Store) readMemoryFile(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path constructed internally
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("filememory: failed to read %s: %w", path, err)
	}

	content := string(data)
	return truncateToMaxLines(content, path), nil
}

// truncateToMaxLines truncates content to maxLines, appending a notice if truncated.
func truncateToMaxLines(content string, path string) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content
	}

	truncated := strings.Join(lines[:maxLines], "\n")
	return truncated + fmt.Sprintf("\n[... truncated at %d lines. Use read tool for full file: %s]", maxLines, path)
}

// globalMemoryPath returns the path to the global MEMORY.md.
func (s *Store) globalMemoryPath() string {
	return filepath.Join(s.baseDir, memoryFileName)
}

// dagMemoryPath returns the path to a DAG-specific MEMORY.md.
func (s *Store) dagMemoryPath(dagName string) string {
	return filepath.Join(s.baseDir, dagSubDir, dagName, memoryFileName)
}

// validateDAGName checks that the DAG name is safe and doesn't escape the base directory.
func (s *Store) validateDAGName(dagName string) error {
	if dagName == "" {
		return errors.New("filememory: dagName cannot be empty")
	}

	// Reject path traversal attempts
	if strings.Contains(dagName, "..") || strings.ContainsAny(dagName, `/\`) {
		return fmt.Errorf("filememory: invalid dagName %q: contains path separator or traversal", dagName)
	}

	// Verify the resolved path stays within baseDir
	resolved := filepath.Join(s.baseDir, dagSubDir, dagName)
	if !strings.HasPrefix(resolved, filepath.Join(s.baseDir, dagSubDir)) {
		return fmt.Errorf("filememory: dagName %q escapes base directory", dagName)
	}

	return nil
}
