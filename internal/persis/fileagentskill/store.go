package fileagentskill

import (
	"bytes"
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
	skillFilename       = "SKILL.md"
	skillDirPermissions = 0750
	filePermissions     = 0600
)

// skillFrontmatter holds the YAML fields in the SKILL.md frontmatter.
// The ID is derived from the directory name, not stored in the file.
type skillFrontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description,omitempty"`
	Version     string   `yaml:"version,omitempty"`
	Author      string   `yaml:"author,omitempty"`
	Tags        []string `yaml:"tags,omitempty"`
}

// Store implements a file-based skill store.
// Skills are stored as directories: {baseDir}/{id}/SKILL.md
// Each SKILL.md contains YAML frontmatter (metadata) and a Markdown body (knowledge).
// Thread-safe through internal locking.
type Store struct {
	baseDir string

	mu     sync.RWMutex
	byID   map[string]string // skill ID -> directory path
	byName map[string]string // skill name -> skill ID
}

// New creates a new file-based skill store.
// The baseDir is the directory where skill directories are stored.
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
		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(s.baseDir, entry.Name())
		skillPath := filepath.Join(dirPath, skillFilename)

		if _, err := os.Stat(skillPath); err != nil {
			continue
		}

		skill, err := loadSkillFromFile(skillPath, entry.Name())
		if err != nil {
			slog.Warn("Failed to load skill file during index rebuild",
				slog.String("file", skillPath),
				slog.String("error", err.Error()))
			continue
		}

		s.byID[skill.ID] = dirPath
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

// parseSkillFile parses a SKILL.md file into an agent.Skill.
// The file format is YAML frontmatter between --- delimiters, followed by markdown body.
func parseSkillFile(data []byte, id string) (*agent.Skill, error) {
	content := string(data)

	if !strings.HasPrefix(content, "---\n") {
		return nil, fmt.Errorf("missing frontmatter delimiter")
	}

	// Find the closing ---
	rest := content[4:] // skip opening "---\n"
	closingIdx := strings.Index(rest, "\n---\n")
	if closingIdx == -1 {
		// Try ending with just "---" at end of file (no trailing newline after body)
		closingIdx = strings.Index(rest, "\n---")
		if closingIdx == -1 {
			return nil, fmt.Errorf("missing closing frontmatter delimiter")
		}
	}

	frontmatterStr := rest[:closingIdx]
	// Body starts after the closing "\n---\n"
	bodyStart := closingIdx + 5 // len("\n---\n")
	if bodyStart > len(rest) {
		bodyStart = len(rest)
	}
	body := rest[bodyStart:]

	var fm skillFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatterStr), &fm); err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	return &agent.Skill{
		ID:          id,
		Name:        fm.Name,
		Description: fm.Description,
		Version:     fm.Version,
		Author:      fm.Author,
		Tags:        fm.Tags,
		Type:        agent.SkillTypeCustom,
		Knowledge:   strings.TrimRight(body, "\n"),
	}, nil
}

// marshalSkillFile produces the SKILL.md content from an agent.Skill.
func marshalSkillFile(skill *agent.Skill) ([]byte, error) {
	fm := skillFrontmatter{
		Name:        skill.Name,
		Description: skill.Description,
		Version:     skill.Version,
		Author:      skill.Author,
		Tags:        skill.Tags,
	}

	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal frontmatter: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(fmBytes)
	buf.WriteString("---\n")
	if skill.Knowledge != "" {
		buf.WriteString(skill.Knowledge)
		buf.WriteString("\n")
	}

	return buf.Bytes(), nil
}

// loadSkillFromFile reads and parses a SKILL.md file.
func loadSkillFromFile(filePath, id string) (*agent.Skill, error) {
	data, err := os.ReadFile(filePath) //nolint:gosec // filePath is constructed internally
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	skill, err := parseSkillFile(data, id)
	if err != nil {
		return nil, fmt.Errorf("failed to parse skill file %s: %w", filePath, err)
	}

	return skill, nil
}

// writeSkillToFile writes a skill to a SKILL.md file atomically.
func writeSkillToFile(filePath string, skill *agent.Skill) error {
	data, err := marshalSkillFile(skill)
	if err != nil {
		return fmt.Errorf("fileagentskill: failed to marshal skill: %w", err)
	}
	if err := fileutil.WriteFileAtomic(filePath, data, filePermissions); err != nil {
		return fmt.Errorf("fileagentskill: %w", err)
	}
	return nil
}

// skillDirPath returns the directory path for a skill ID.
// Callers must validate the ID before calling this method.
func (s *Store) skillDirPath(id string) (string, error) {
	p := filepath.Join(s.baseDir, id)
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

	dirPath, err := s.skillDirPath(skill.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dirPath, skillDirPermissions); err != nil {
		return fmt.Errorf("fileagentskill: failed to create skill directory: %w", err)
	}

	filePath := filepath.Join(dirPath, skillFilename)
	if err := writeSkillToFile(filePath, skill); err != nil {
		// Clean up directory on write failure
		_ = os.RemoveAll(dirPath)
		return err
	}

	s.byID[skill.ID] = dirPath
	s.byName[skill.Name] = skill.ID

	return nil
}

// GetByID retrieves a skill by its unique ID.
func (s *Store) GetByID(_ context.Context, id string) (*agent.Skill, error) {
	if err := agent.ValidateSkillID(id); err != nil {
		return nil, err
	}

	s.mu.RLock()
	dirPath, exists := s.byID[id]
	s.mu.RUnlock()

	if !exists {
		return nil, agent.ErrSkillNotFound
	}

	filePath := filepath.Join(dirPath, skillFilename)
	skill, err := loadSkillFromFile(filePath, id)
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
	entries := make([]struct {
		id      string
		dirPath string
	}, 0, len(s.byID))
	for id, dp := range s.byID {
		entries = append(entries, struct {
			id      string
			dirPath string
		}{id, dp})
	}
	s.mu.RUnlock()

	skills := make([]*agent.Skill, 0, len(entries))
	for _, e := range entries {
		filePath := filepath.Join(e.dirPath, skillFilename)
		skill, err := loadSkillFromFile(filePath, e.id)
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

	dirPath, exists := s.byID[skill.ID]
	if !exists {
		return agent.ErrSkillNotFound
	}

	filePath := filepath.Join(dirPath, skillFilename)
	existing, err := loadSkillFromFile(filePath, skill.ID)
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

	dirPath, exists := s.byID[id]
	if !exists {
		return agent.ErrSkillNotFound
	}

	if err := os.RemoveAll(dirPath); err != nil {
		return fmt.Errorf("fileagentskill: failed to delete skill directory: %w", err)
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
