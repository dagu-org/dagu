// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package filememory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/dagucloud/dagu/internal/agent"
	"github.com/dagucloud/dagu/internal/cmn/fileutil"
)

// Verify Store implements agent.MemoryStore at compile time.
var (
	_ agent.MemoryStore            = (*Store)(nil)
	_ agent.AutopilotDocumentStore = (*Store)(nil)
)

const (
	agentMemoryDir        = "memory"
	dagSubDir             = "dags"
	autopilotSubDir       = "autopilot"
	legacyAutopilotSubDir = "automata"
	memoryFileName        = "MEMORY.md"
	soulFileName          = "SOUL.md"
	maxLines              = 200
	dirPermissions        = 0750
	filePermissions       = 0600
)

var autopilotNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_]*$`)

var autopilotDocumentNames = map[string]struct{}{
	memoryFileName: {},
	soulFileName:   {},
}

// Store implements a file-based memory store for the agent.
// Memory files are stored under {dagsDir}/memory/.
// Thread-safe through internal locking.
type Store struct {
	baseDir   string // {dagsDir}/memory
	mu        sync.RWMutex
	fileCache *fileutil.Cache[string]
}

// Option is a functional option for configuring the Store.
type Option func(*Store)

// WithFileCache sets the file cache for memory content.
func WithFileCache(cache *fileutil.Cache[string]) Option {
	return func(s *Store) {
		s.fileCache = cache
	}
}

// New creates a new file-based agent memory store.
// The dagsDir is the DAGs directory (e.g., ~/.config/dagu/dags).
// The memory files will be stored under {dagsDir}/memory/.
func New(dagsDir string, opts ...Option) (*Store, error) {
	if dagsDir == "" {
		return nil, errors.New("filememory: dagsDir cannot be empty")
	}

	baseDir := filepath.Join(dagsDir, agentMemoryDir)
	if err := os.MkdirAll(baseDir, dirPermissions); err != nil {
		return nil, fmt.Errorf("filememory: failed to create directory %s: %w", baseDir, err)
	}

	s := &Store{baseDir: baseDir}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// LoadGlobalMemory reads the global MEMORY.md, truncated to maxLines.
// Returns empty string if the file does not exist.
func (s *Store) LoadGlobalMemory(_ context.Context) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.fileCache != nil {
		return s.loadMemoryWithCache(s.globalMemoryPath())
	}
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

	path := s.dagMemoryPath(dagName)
	if s.fileCache != nil {
		return s.loadMemoryWithCache(path)
	}
	return s.readMemoryFile(path)
}

// LoadAutopilotMemory reads the MEMORY.md for a specific Autopilot, truncated to maxLines.
// Returns empty string if the file does not exist.
func (s *Store) LoadAutopilotMemory(ctx context.Context, autopilotName string) (string, error) {
	return s.LoadAutopilotDocument(ctx, autopilotName, memoryFileName)
}

// LoadAutopilotDocument reads an allowed Autopilot document, truncated to maxLines.
// Returns empty string if the file does not exist.
func (s *Store) LoadAutopilotDocument(_ context.Context, autopilotName, document string) (string, error) {
	path, err := s.AutopilotDocumentPath(autopilotName, document)
	if err != nil {
		return "", err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.fileCache != nil {
		return s.loadMemoryWithCache(path)
	}
	return s.readMemoryFile(path)
}

// SaveGlobalMemory writes content to the global MEMORY.md atomically.
func (s *Store) SaveGlobalMemory(_ context.Context, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.globalMemoryPath()
	if err := fileutil.WriteFileAtomic(path, []byte(content), filePermissions); err != nil {
		return err
	}
	if s.fileCache != nil {
		s.fileCache.Invalidate(path)
	}
	return nil
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

	path := s.dagMemoryPath(dagName)
	if err := fileutil.WriteFileAtomic(path, []byte(content), filePermissions); err != nil {
		return err
	}
	if s.fileCache != nil {
		s.fileCache.Invalidate(path)
	}
	return nil
}

// SaveAutopilotMemory writes content to an Autopilot-specific MEMORY.md atomically.
func (s *Store) SaveAutopilotMemory(ctx context.Context, autopilotName string, content string) error {
	return s.SaveAutopilotDocument(ctx, autopilotName, memoryFileName, content)
}

// SaveAutopilotDocument writes an allowed Autopilot document atomically.
func (s *Store) SaveAutopilotDocument(_ context.Context, autopilotName, document, content string) error {
	path, err := s.AutopilotDocumentPath(autopilotName, document)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), dirPermissions); err != nil {
		return fmt.Errorf("filememory: failed to create autopilot document directory: %w", err)
	}

	if err := fileutil.WriteFileAtomic(path, []byte(content), filePermissions); err != nil {
		return err
	}
	if s.fileCache != nil {
		s.fileCache.Invalidate(path)
	}
	return nil
}

// MemoryDir returns the root memory directory path.
func (s *Store) MemoryDir() string {
	return s.baseDir
}

// AutopilotMemoryPath returns the resolved path to an Autopilot-specific MEMORY.md.
func (s *Store) AutopilotMemoryPath(autopilotName string) (string, error) {
	return s.AutopilotDocumentPath(autopilotName, memoryFileName)
}

// AutopilotDocumentPath returns the resolved path to an allowed Autopilot document.
func (s *Store) AutopilotDocumentPath(autopilotName, document string) (string, error) {
	if err := s.validateAutopilotName(autopilotName); err != nil {
		return "", err
	}
	if err := validateAutopilotDocumentName(document); err != nil {
		return "", err
	}
	return s.resolveAutopilotDocumentPath(autopilotName, document), nil
}

// ListDAGMemories returns the names of all DAGs that have memory files.
func (s *Store) ListDAGMemories(_ context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dagsDir := filepath.Join(s.baseDir, dagSubDir)
	entries, err := os.ReadDir(dagsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("filememory: failed to read dags directory: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		memPath := filepath.Join(dagsDir, entry.Name(), memoryFileName)
		if _, err := os.Stat(memPath); err == nil {
			names = append(names, entry.Name())
		}
	}

	return names, nil
}

// DeleteGlobalMemory removes the global MEMORY.md file.
func (s *Store) DeleteGlobalMemory(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.globalMemoryPath()
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("filememory: failed to delete global memory: %w", err)
	}
	if s.fileCache != nil {
		s.fileCache.Invalidate(path)
	}
	return nil
}

// DeleteDAGMemory removes a DAG-specific MEMORY.md file and its directory.
func (s *Store) DeleteDAGMemory(_ context.Context, dagName string) error {
	if err := s.validateDAGName(dagName); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	dagDir := filepath.Join(s.baseDir, dagSubDir, dagName)
	memPath := s.dagMemoryPath(dagName)
	err := os.RemoveAll(dagDir)
	if err != nil {
		return fmt.Errorf("filememory: failed to delete DAG memory directory: %w", err)
	}
	if s.fileCache != nil {
		s.fileCache.Invalidate(memPath)
	}
	return nil
}

// DeleteAutopilotMemory removes an Autopilot-specific MEMORY.md file.
func (s *Store) DeleteAutopilotMemory(ctx context.Context, autopilotName string) error {
	return s.DeleteAutopilotDocument(ctx, autopilotName, memoryFileName)
}

// DeleteAutopilotDocument removes an allowed Autopilot document and removes the
// containing Autopilot document directory when it becomes empty.
func (s *Store) DeleteAutopilotDocument(_ context.Context, autopilotName, document string) error {
	path, err := s.AutopilotDocumentPath(autopilotName, document)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("filememory: failed to delete autopilot document: %w", err)
	}
	if s.fileCache != nil {
		s.fileCache.Invalidate(path)
	}
	if err := removeEmptyDir(filepath.Dir(path)); err != nil {
		return fmt.Errorf("filememory: failed to remove empty autopilot document directory: %w", err)
	}
	return nil
}

func (s *Store) loadMemoryWithCache(path string) (string, error) {
	content, err := s.fileCache.LoadLatest(path, func() (string, error) {
		return s.readMemoryFile(path)
	})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || os.IsNotExist(err) {
			s.fileCache.Invalidate(path)
			return "", nil
		}
		return "", err
	}
	return content, nil
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
	return truncateToMaxLines(string(data), path), nil
}

// truncateToMaxLines truncates content to maxLines, appending a notice if truncated.
func truncateToMaxLines(content, path string) string {
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

// autopilotMemoryPath returns the path to an Autopilot-specific MEMORY.md.
func (s *Store) autopilotMemoryPath(autopilotName string) string {
	return s.autopilotDocumentPath(autopilotName, memoryFileName)
}

func (s *Store) autopilotDocumentPath(autopilotName, document string) string {
	return filepath.Join(s.baseDir, autopilotSubDir, autopilotName, document)
}

func (s *Store) legacyAutopilotDocumentPath(autopilotName, document string) string {
	return filepath.Join(s.baseDir, legacyAutopilotSubDir, autopilotName, document)
}

func (s *Store) resolveAutopilotDocumentPath(autopilotName, document string) string {
	newPath := s.autopilotDocumentPath(autopilotName, document)
	if pathExists(newPath) {
		return newPath
	}
	legacyPath := s.legacyAutopilotDocumentPath(autopilotName, document)
	if pathExists(legacyPath) || pathExists(filepath.Dir(legacyPath)) {
		return legacyPath
	}
	return newPath
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

func (s *Store) validateAutopilotName(autopilotName string) error {
	if autopilotName == "" {
		return errors.New("filememory: autopilotName cannot be empty")
	}
	if !autopilotNamePattern.MatchString(autopilotName) {
		return fmt.Errorf("filememory: invalid autopilotName %q", autopilotName)
	}
	return nil
}

func validateAutopilotDocumentName(document string) error {
	if _, ok := autopilotDocumentNames[document]; ok {
		return nil
	}
	return fmt.Errorf("filememory: invalid autopilot document %q", document)
}

func removeEmptyDir(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(entries) > 0 {
		return nil
	}
	return os.Remove(path)
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
