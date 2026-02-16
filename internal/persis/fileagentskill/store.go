package fileagentskill

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/goccy/go-yaml"
)

// Verify Store implements agent.SkillStore at compile time.
var _ agent.SkillStore = (*Store)(nil)

const (
	skillFileExtension  = ".yaml"
	skillDirPermissions = 0750
	filePermissions     = 0600
	dirSkillFilename    = "skill.yaml"
)

// Store implements a file-based skill store.
// Skills are stored as individual YAML files in {baseDir}/{id}.yaml,
// or as {baseDir}/{id}/skill.yaml for directory-based skills.
// Thread-safe through internal locking.
type Store struct {
	baseDir string

	mu     sync.RWMutex
	byID   map[string]string // skill ID -> file path
	byName map[string]string // skill name -> skill ID
}

// New creates a new file-based skill store.
// The baseDir is the directory where skill files are stored.
func New(baseDir string) (*Store, error) {
	if baseDir == "" {
		return nil, errors.New("fileagentskill: baseDir cannot be empty")
	}

	store := &Store{
		baseDir: baseDir,
		byID:    make(map[string]string),
		byName:  make(map[string]string),
	}

	if err := os.MkdirAll(baseDir, skillDirPermissions); err != nil {
		return nil, fmt.Errorf("fileagentskill: failed to create directory %s: %w", baseDir, err)
	}

	if err := store.rebuildIndex(); err != nil {
		return nil, fmt.Errorf("fileagentskill: failed to build index: %w", err)
	}

	return store, nil
}

// rebuildIndex scans the directory and rebuilds the in-memory index.
func (s *Store) rebuildIndex() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.byID = make(map[string]string)
	s.byName = make(map[string]string)

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", s.baseDir, err)
	}

	for _, entry := range entries {
		var filePath string

		switch {
		case !entry.IsDir() && filepath.Ext(entry.Name()) == skillFileExtension:
			// Single-file skill: {id}.yaml
			filePath = filepath.Join(s.baseDir, entry.Name())
		case entry.IsDir():
			// Directory-based skill: {id}/skill.yaml
			candidate := filepath.Join(s.baseDir, entry.Name(), dirSkillFilename)
			if _, err := os.Stat(candidate); err == nil {
				filePath = candidate
			}
		}

		if filePath == "" {
			continue
		}

		skill, err := loadSkillFromFile(filePath)
		if err != nil {
			slog.Warn("Failed to load skill file during index rebuild",
				slog.String("file", filePath),
				slog.String("error", err.Error()))
			continue
		}

		s.byID[skill.ID] = filePath
		if existingID, exists := s.byName[skill.Name]; exists {
			slog.Warn("Duplicate skill name in store, last file wins",
				slog.String("name", skill.Name),
				slog.String("existingID", existingID),
				slog.String("newID", skill.ID))
		}
		s.byName[skill.Name] = skill.ID
	}

	return nil
}

// loadSkillFromFile reads and parses a skill from a YAML file.
func loadSkillFromFile(filePath string) (*agent.Skill, error) {
	data, err := os.ReadFile(filePath) //nolint:gosec // filePath is constructed internally
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	var skill agent.Skill
	if err := yaml.Unmarshal(data, &skill); err != nil {
		return nil, fmt.Errorf("failed to parse skill file %s: %w", filePath, err)
	}

	return &skill, nil
}

// writeSkillToFile writes a skill to a YAML file atomically.
func writeSkillToFile(filePath string, skill *agent.Skill) error {
	data, err := yaml.Marshal(skill)
	if err != nil {
		return fmt.Errorf("fileagentskill: failed to marshal skill: %w", err)
	}
	if err := fileutil.WriteFileAtomic(filePath, data, filePermissions); err != nil {
		return fmt.Errorf("fileagentskill: %w", err)
	}
	return nil
}

// skillFilePath returns the file path for a skill ID.
// Callers must validate the ID before calling this method.
func (s *Store) skillFilePath(id string) (string, error) {
	p := filepath.Join(s.baseDir, id+skillFileExtension)
	// Defense-in-depth: ensure the resolved path stays within baseDir
	if !strings.HasPrefix(p, filepath.Clean(s.baseDir)+string(filepath.Separator)) {
		return "", fmt.Errorf("fileagentskill: path traversal detected for id %q", id)
	}
	return p, nil
}

// Create stores a new skill.
func (s *Store) Create(_ context.Context, skill *agent.Skill) error {
	if skill == nil {
		return errors.New("fileagentskill: skill cannot be nil")
	}
	if err := agent.ValidateSkillID(skill.ID); err != nil {
		return err
	}
	if skill.Name == "" {
		return errors.New("fileagentskill: skill name is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byID[skill.ID]; exists {
		return agent.ErrSkillAlreadyExists
	}
	if _, exists := s.byName[skill.Name]; exists {
		return agent.ErrSkillNameAlreadyExists
	}

	filePath, err := s.skillFilePath(skill.ID)
	if err != nil {
		return err
	}
	if err := writeSkillToFile(filePath, skill); err != nil {
		return err
	}

	s.byID[skill.ID] = filePath
	s.byName[skill.Name] = skill.ID

	return nil
}

// GetByID retrieves a skill by its unique ID.
func (s *Store) GetByID(_ context.Context, id string) (*agent.Skill, error) {
	if err := agent.ValidateSkillID(id); err != nil {
		return nil, err
	}

	s.mu.RLock()
	filePath, exists := s.byID[id]
	s.mu.RUnlock()

	if !exists {
		return nil, agent.ErrSkillNotFound
	}

	skill, err := loadSkillFromFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, agent.ErrSkillNotFound
		}
		return nil, fmt.Errorf("fileagentskill: failed to load skill %s: %w", id, err)
	}

	return skill, nil
}

// List returns all skills, sorted by name.
func (s *Store) List(_ context.Context) ([]*agent.Skill, error) {
	s.mu.RLock()
	filePaths := make([]string, 0, len(s.byID))
	for _, fp := range s.byID {
		filePaths = append(filePaths, fp)
	}
	s.mu.RUnlock()

	skills := make([]*agent.Skill, 0, len(filePaths))
	for _, fp := range filePaths {
		skill, err := loadSkillFromFile(fp)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("fileagentskill: failed to load skill: %w", err)
		}
		skills = append(skills, skill)
	}

	slices.SortFunc(skills, func(a, b *agent.Skill) int {
		return strings.Compare(a.Name, b.Name)
	})

	return skills, nil
}

// Update modifies an existing skill.
func (s *Store) Update(_ context.Context, skill *agent.Skill) error {
	if skill == nil {
		return errors.New("fileagentskill: skill cannot be nil")
	}
	if err := agent.ValidateSkillID(skill.ID); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath, exists := s.byID[skill.ID]
	if !exists {
		return agent.ErrSkillNotFound
	}

	existing, err := loadSkillFromFile(filePath)
	if err != nil {
		return fmt.Errorf("fileagentskill: failed to load existing skill: %w", err)
	}

	nameChanged := skill.Name != "" && existing.Name != skill.Name
	if nameChanged {
		if takenByID, taken := s.byName[skill.Name]; taken && takenByID != skill.ID {
			return agent.ErrSkillAlreadyExists
		}
	}

	if err := writeSkillToFile(filePath, skill); err != nil {
		return err
	}

	if nameChanged {
		delete(s.byName, existing.Name)
		s.byName[skill.Name] = skill.ID
	}

	return nil
}

// Delete removes a skill by its ID.
func (s *Store) Delete(_ context.Context, id string) error {
	if err := agent.ValidateSkillID(id); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath, exists := s.byID[id]
	if !exists {
		return agent.ErrSkillNotFound
	}

	if err := os.Remove(filePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("fileagentskill: failed to delete skill file: %w", err)
	}

	delete(s.byID, id)

	// Clean up name index by reverse lookup.
	for name, skillID := range s.byName {
		if skillID == id {
			delete(s.byName, name)
			break
		}
	}

	return nil
}
