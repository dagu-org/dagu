package fileagentconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

const (
	// configFileName is the name of the agent config file.
	configFileName = "config.json"
	// agentDirName is the directory name for agent config.
	agentDirName = "agent"
	// dirPermissions is the permission mode for directories.
	dirPermissions = 0750
	// filePermissions is the permission mode for config files.
	filePermissions = 0600
)

// Environment variable names for agent configuration overrides.
const (
	envAgentEnabled     = "DAGU_AGENT_ENABLED"
	envAgentLLMProvider = "DAGU_AGENT_LLM_PROVIDER"
	envAgentLLMModel    = "DAGU_AGENT_LLM_MODEL"
	envAgentLLMAPIKey   = "DAGU_AGENT_LLM_API_KEY"
	envAgentLLMBaseURL  = "DAGU_AGENT_LLM_BASE_URL"
)

// Store implements a file-based singleton store for agent configuration.
// The config is stored as a JSON file at {dataDir}/agent/config.json.
// Thread-safe through internal locking.
type Store struct {
	baseDir string
	mu      sync.RWMutex
}

// New creates a new file-based agent config store.
// The dataDir is the base data directory (e.g., DAGU_HOME/data).
// The config will be stored at {dataDir}/agent/config.json.
func New(dataDir string) (*Store, error) {
	if dataDir == "" {
		return nil, errors.New("fileagentconfig: dataDir cannot be empty")
	}

	baseDir := filepath.Join(dataDir, agentDirName)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(baseDir, dirPermissions); err != nil {
		return nil, fmt.Errorf("fileagentconfig: failed to create directory %s: %w", baseDir, err)
	}

	return &Store{
		baseDir: baseDir,
	}, nil
}

// Load reads the agent configuration from the JSON file.
// If the file doesn't exist, returns the default configuration.
// Environment variables override values from the JSON file.
// Priority: Environment variables > JSON file > Defaults
func (s *Store) Load(_ context.Context) (*AgentConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	configPath := s.configPath()

	// Start with default config
	cfg := DefaultConfig()

	// Try to read from file
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No file exists, apply env overrides to defaults and return
			s.applyEnvOverrides(cfg)
			return cfg, nil
		}
		return nil, fmt.Errorf("fileagentconfig: failed to read config file: %w", err)
	}

	// Parse JSON
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("fileagentconfig: failed to parse config file: %w", err)
	}

	// Apply environment variable overrides
	s.applyEnvOverrides(cfg)

	return cfg, nil
}

// Save writes the agent configuration to the JSON file.
// Uses atomic write (temp file + rename) to prevent corruption.
func (s *Store) Save(_ context.Context, cfg *AgentConfig) error {
	if cfg == nil {
		return errors.New("fileagentconfig: config cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	configPath := s.configPath()

	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("fileagentconfig: failed to marshal config: %w", err)
	}

	// Write to temp file first
	tempFile := configPath + ".tmp"
	if err := os.WriteFile(tempFile, data, filePermissions); err != nil {
		return fmt.Errorf("fileagentconfig: failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, configPath); err != nil {
		// Clean up temp file on failure
		_ = os.Remove(tempFile)
		return fmt.Errorf("fileagentconfig: failed to rename temp file: %w", err)
	}

	return nil
}

// Exists returns true if the config file exists.
func (s *Store) Exists() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, err := os.Stat(s.configPath())
	return err == nil
}

// configPath returns the full path to the config file.
func (s *Store) configPath() string {
	return filepath.Join(s.baseDir, configFileName)
}

// applyEnvOverrides applies environment variable overrides to the config.
// Environment variables take precedence over JSON file values.
func (s *Store) applyEnvOverrides(cfg *AgentConfig) {
	// DAGU_AGENT_ENABLED
	if v := os.Getenv(envAgentEnabled); v != "" {
		if enabled, err := strconv.ParseBool(v); err == nil {
			cfg.Enabled = enabled
		}
	}

	// DAGU_AGENT_LLM_PROVIDER
	if v := os.Getenv(envAgentLLMProvider); v != "" {
		cfg.LLM.Provider = v
	}

	// DAGU_AGENT_LLM_MODEL
	if v := os.Getenv(envAgentLLMModel); v != "" {
		cfg.LLM.Model = v
	}

	// DAGU_AGENT_LLM_API_KEY
	if v := os.Getenv(envAgentLLMAPIKey); v != "" {
		cfg.LLM.APIKey = v
	}

	// DAGU_AGENT_LLM_BASE_URL
	if v := os.Getenv(envAgentLLMBaseURL); v != "" {
		cfg.LLM.BaseURL = v
	}
}
