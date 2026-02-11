package fileagentmodel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
)

// Verify Store implements agent.ModelStore at compile time.
var _ agent.ModelStore = (*Store)(nil)

const (
	modelFileExtension  = ".json"
	modelDirPermissions = 0750
	filePermissions     = 0600
)

// Store implements a file-based model store.
// Models are stored as individual JSON files in {baseDir}/{id}.json.
// Thread-safe through internal locking.
type Store struct {
	baseDir string

	mu     sync.RWMutex
	byID   map[string]string // model ID -> file path
	byName map[string]string // model name -> model ID
}

// Option is a functional option for configuring the Store.
type Option func(*Store)

// New creates a new file-based model store.
// The baseDir is the directory where model files are stored.
func New(baseDir string, opts ...Option) (*Store, error) {
	if baseDir == "" {
		return nil, errors.New("fileagentmodel: baseDir cannot be empty")
	}

	store := &Store{
		baseDir: baseDir,
		byID:    make(map[string]string),
		byName:  make(map[string]string),
	}

	for _, opt := range opts {
		opt(store)
	}

	if err := os.MkdirAll(baseDir, modelDirPermissions); err != nil {
		return nil, fmt.Errorf("fileagentmodel: failed to create directory %s: %w", baseDir, err)
	}

	if err := store.rebuildIndex(); err != nil {
		return nil, fmt.Errorf("fileagentmodel: failed to build index: %w", err)
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
		if entry.IsDir() || filepath.Ext(entry.Name()) != modelFileExtension {
			continue
		}

		filePath := filepath.Join(s.baseDir, entry.Name())
		model, err := loadModelFromFile(filePath)
		if err != nil {
			slog.Warn("Failed to load model file during index rebuild",
				slog.String("file", filePath),
				slog.String("error", err.Error()))
			continue
		}

		s.byID[model.ID] = filePath
		s.byName[model.Name] = model.ID
	}

	return nil
}

// loadModelFromFile reads and parses a model config from a JSON file.
func loadModelFromFile(filePath string) (*agent.ModelConfig, error) {
	data, err := os.ReadFile(filePath) //nolint:gosec // filePath is constructed internally
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	var model agent.ModelConfig
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, fmt.Errorf("failed to parse model file %s: %w", filePath, err)
	}

	return &model, nil
}

// writeModelToFile writes a model config to a JSON file atomically.
func writeModelToFile(filePath string, model *agent.ModelConfig) error {
	if err := fileutil.WriteJSONAtomic(filePath, model, filePermissions); err != nil {
		return fmt.Errorf("fileagentmodel: %w", err)
	}
	return nil
}

// modelFilePath returns the file path for a model ID.
func (s *Store) modelFilePath(id string) string {
	return filepath.Join(s.baseDir, id+modelFileExtension)
}

// Create stores a new model configuration.
func (s *Store) Create(_ context.Context, model *agent.ModelConfig) error {
	if model == nil {
		return errors.New("fileagentmodel: model cannot be nil")
	}
	if model.ID == "" {
		return agent.ErrInvalidModelID
	}
	if model.Name == "" {
		return errors.New("fileagentmodel: model name is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byID[model.ID]; exists {
		return agent.ErrModelAlreadyExists
	}

	filePath := s.modelFilePath(model.ID)
	if err := writeModelToFile(filePath, model); err != nil {
		return err
	}

	s.byID[model.ID] = filePath
	s.byName[model.Name] = model.ID

	return nil
}

// GetByID retrieves a model configuration by its unique ID.
func (s *Store) GetByID(_ context.Context, id string) (*agent.ModelConfig, error) {
	if id == "" {
		return nil, agent.ErrInvalidModelID
	}

	s.mu.RLock()
	filePath, exists := s.byID[id]
	if !exists {
		s.mu.RUnlock()
		return nil, agent.ErrModelNotFound
	}

	model, err := loadModelFromFile(filePath)
	s.mu.RUnlock()

	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, agent.ErrModelNotFound
		}
		return nil, fmt.Errorf("fileagentmodel: failed to load model %s: %w", id, err)
	}

	return model, nil
}

// List returns all model configurations, sorted by name.
func (s *Store) List(ctx context.Context) ([]*agent.ModelConfig, error) {
	s.mu.RLock()
	ids := make([]string, 0, len(s.byID))
	for id := range s.byID {
		ids = append(ids, id)
	}
	s.mu.RUnlock()

	models := make([]*agent.ModelConfig, 0, len(ids))
	for _, id := range ids {
		model, err := s.GetByID(ctx, id)
		if err != nil {
			if errors.Is(err, agent.ErrModelNotFound) {
				continue
			}
			return nil, err
		}
		models = append(models, model)
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].Name < models[j].Name
	})

	return models, nil
}

// Update modifies an existing model configuration.
func (s *Store) Update(_ context.Context, model *agent.ModelConfig) error {
	if model == nil {
		return errors.New("fileagentmodel: model cannot be nil")
	}
	if model.ID == "" {
		return agent.ErrInvalidModelID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath, exists := s.byID[model.ID]
	if !exists {
		return agent.ErrModelNotFound
	}

	existing, err := loadModelFromFile(filePath)
	if err != nil {
		return fmt.Errorf("fileagentmodel: failed to load existing model: %w", err)
	}

	// If name changed, check for conflicts
	if existing.Name != model.Name && model.Name != "" {
		if existingID, taken := s.byName[model.Name]; taken && existingID != model.ID {
			return agent.ErrModelAlreadyExists
		}
	}

	if err := writeModelToFile(filePath, model); err != nil {
		return err
	}

	// Update name index after successful write
	if existing.Name != model.Name && model.Name != "" {
		delete(s.byName, existing.Name)
		s.byName[model.Name] = model.ID
	}

	return nil
}

// Delete removes a model configuration by its ID.
func (s *Store) Delete(_ context.Context, id string) error {
	if id == "" {
		return agent.ErrInvalidModelID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath, exists := s.byID[id]
	if !exists {
		return agent.ErrModelNotFound
	}

	model, err := loadModelFromFile(filePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("fileagentmodel: failed to load model for deletion: %w", err)
	}

	if err := os.Remove(filePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("fileagentmodel: failed to delete model file: %w", err)
	}

	delete(s.byID, id)
	if model != nil {
		delete(s.byName, model.Name)
	} else {
		var nameToDelete string
		for name, modelID := range s.byName {
			if modelID == id {
				nameToDelete = name
				break
			}
		}
		if nameToDelete != "" {
			delete(s.byName, nameToDelete)
		}
	}

	return nil
}
