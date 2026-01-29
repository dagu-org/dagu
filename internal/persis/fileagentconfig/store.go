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
	configFileName  = "config.json"
	agentDirName    = "agent"
	dirPermissions  = 0750
	filePermissions = 0600
)

// Environment variable names for agent configuration overrides.
const (
	envAgentEnabled     = "DAGU_AGENT_ENABLED"
	envAgentLLMProvider = "DAGU_AGENT_LLM_PROVIDER"
	envAgentLLMModel    = "DAGU_AGENT_LLM_MODEL"
	envAgentLLMAPIKey   = "DAGU_AGENT_LLM_API_KEY" //nolint:gosec // constant name, not a credential
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
	if err := os.MkdirAll(baseDir, dirPermissions); err != nil {
		return nil, fmt.Errorf("fileagentconfig: failed to create directory %s: %w", baseDir, err)
	}

	return &Store{baseDir: baseDir}, nil
}

// Load reads the agent configuration from the JSON file.
// If the file doesn't exist, returns the default configuration.
// Priority: Environment variables > JSON file > Defaults
func (s *Store) Load(_ context.Context) (*AgentConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cfg := DefaultConfig()

	data, err := os.ReadFile(s.configPath())
	if err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("fileagentconfig: failed to parse config file: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("fileagentconfig: failed to read config file: %w", err)
	}

	applyEnvOverrides(cfg)

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

	return s.writeConfigToFile(s.configPath(), cfg)
}

// writeConfigToFile writes the config to a JSON file atomically.
func (s *Store) writeConfigToFile(filePath string, cfg *AgentConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("fileagentconfig: failed to marshal config: %w", err)
	}

	tempPath := filePath + ".tmp"
	if err := os.WriteFile(tempPath, data, filePermissions); err != nil {
		return fmt.Errorf("fileagentconfig: failed to write temp file: %w", err)
	}

	if err := os.Rename(tempPath, filePath); err != nil {
		_ = os.Remove(tempPath)
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
func applyEnvOverrides(cfg *AgentConfig) {
	if v := os.Getenv(envAgentEnabled); v != "" {
		if enabled, err := strconv.ParseBool(v); err == nil {
			cfg.Enabled = enabled
		}
	}

	applyStringEnvOverride(envAgentLLMProvider, &cfg.LLM.Provider)
	applyStringEnvOverride(envAgentLLMModel, &cfg.LLM.Model)
	applyStringEnvOverride(envAgentLLMAPIKey, &cfg.LLM.APIKey)
	applyStringEnvOverride(envAgentLLMBaseURL, &cfg.LLM.BaseURL)
}

// applyStringEnvOverride sets the target value if the environment variable is non-empty.
func applyStringEnvOverride(envVar string, target *string) {
	if v := os.Getenv(envVar); v != "" {
		*target = v
	}
}
