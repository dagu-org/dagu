package llm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	llmpkg "github.com/dagu-org/dagu/internal/llm"
)

const historySuffix = ".history.json"

// HistoryFile handles reading and writing conversation history files.
type HistoryFile struct {
	dir      string
	stepName string
}

// NewHistoryFile creates a new HistoryFile for a step.
func NewHistoryFile(dir, stepName string) *HistoryFile {
	return &HistoryFile{
		dir:      dir,
		stepName: stepName,
	}
}

// Path returns the full path to the history file.
func (h *HistoryFile) Path() string {
	return filepath.Join(h.dir, h.stepName+historySuffix)
}

// Read loads the conversation history from the file.
// Returns an empty slice if the file doesn't exist.
func (h *HistoryFile) Read() ([]llmpkg.Message, error) {
	path := h.Path()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read history file: %w", err)
	}

	if len(data) == 0 {
		return nil, nil
	}

	var messages []llmpkg.Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, fmt.Errorf("failed to parse history file: %w", err)
	}

	return messages, nil
}

// Write saves the conversation history to the file.
func (h *HistoryFile) Write(messages []llmpkg.Message) error {
	path := h.Path()

	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write history file: %w", err)
	}

	return nil
}

// ReadDependentHistory reads history from a dependent step.
func ReadDependentHistory(dir, dependentStepName string) ([]llmpkg.Message, error) {
	hf := NewHistoryFile(dir, dependentStepName)
	return hf.Read()
}
