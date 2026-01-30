package fileagentconfig

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/llm"
)

// Verify Store implements agent.ConfigStore at compile time.
var _ agent.ConfigStore = (*Store)(nil)

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
	baseDir       string
	mu            sync.RWMutex
	configCache   *fileutil.Cache[*agent.Config]
	providerCache *providerCache
}

// Option is a functional option for configuring the Store.
type Option func(*Store)

// WithConfigCache sets the config cache for the store.
func WithConfigCache(cache *fileutil.Cache[*agent.Config]) Option {
	return func(s *Store) {
		s.configCache = cache
	}
}

// providerCache caches the LLM provider by config hash.
type providerCache struct {
	mu       sync.RWMutex
	provider llm.Provider
	model    string
	cfgHash  string
}

func newProviderCache() *providerCache {
	return &providerCache{}
}

func (c *providerCache) get(llmCfg agent.LLMConfig) (llm.Provider, string, error) {
	hash := hashLLMConfig(llmCfg)

	c.mu.RLock()
	if c.cfgHash == hash && c.provider != nil {
		defer c.mu.RUnlock()
		return c.provider, c.model, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if c.cfgHash == hash && c.provider != nil {
		return c.provider, c.model, nil
	}

	provider, err := createLLMProvider(llmCfg)
	if err != nil {
		return nil, "", err
	}

	c.provider = provider
	c.model = llmCfg.Model
	c.cfgHash = hash
	return provider, c.model, nil
}

// hashLLMConfig creates a hash of the LLM config for cache invalidation.
func hashLLMConfig(cfg agent.LLMConfig) string {
	data := fmt.Sprintf("%s:%s:%s:%s", cfg.Provider, cfg.Model, cfg.APIKey, cfg.BaseURL)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8])
}

// createLLMProvider creates an LLM provider from the config.
func createLLMProvider(agentCfg agent.LLMConfig) (llm.Provider, error) {
	providerType, err := llm.ParseProviderType(agentCfg.Provider)
	if err != nil {
		return nil, fmt.Errorf("invalid LLM provider: %w", err)
	}

	cfg := llm.DefaultConfig()
	cfg.APIKey = cmp.Or(agentCfg.APIKey, cfg.APIKey)
	cfg.BaseURL = cmp.Or(agentCfg.BaseURL, cfg.BaseURL)

	return llm.NewProvider(providerType, cfg)
}

// New creates a new file-based agent config store.
// The dataDir is the base data directory (e.g., DAGU_HOME/data).
// The config will be stored at {dataDir}/agent/config.json.
func New(dataDir string, opts ...Option) (*Store, error) {
	if dataDir == "" {
		return nil, errors.New("fileagentconfig: dataDir cannot be empty")
	}

	baseDir := filepath.Join(dataDir, agentDirName)
	if err := os.MkdirAll(baseDir, dirPermissions); err != nil {
		return nil, fmt.Errorf("fileagentconfig: failed to create directory %s: %w", baseDir, err)
	}

	s := &Store{
		baseDir:       baseDir,
		providerCache: newProviderCache(),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

// Load reads the agent configuration from the JSON file.
// If the file doesn't exist, returns the default configuration.
// Priority: Environment variables > JSON file > Defaults
// Uses cache if available to avoid reading file on every request.
func (s *Store) Load(_ context.Context) (*agent.Config, error) {
	if s.configCache != nil {
		return s.configCache.LoadLatest(s.configPath(), s.loadFromFile)
	}
	return s.loadFromFile()
}

// loadFromFile reads config directly from file.
func (s *Store) loadFromFile() (*agent.Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cfg := agent.DefaultConfig()

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

// IsEnabled returns whether the agent is enabled.
// Reads from cache if available.
func (s *Store) IsEnabled(ctx context.Context) bool {
	cfg, err := s.Load(ctx)
	if err != nil {
		return false
	}
	return cfg.Enabled
}

// GetProvider returns the cached LLM provider and model.
// Creates the provider if config has changed since last call.
func (s *Store) GetProvider(ctx context.Context) (llm.Provider, string, error) {
	cfg, err := s.Load(ctx)
	if err != nil {
		return nil, "", err
	}
	if !cfg.Enabled {
		return nil, "", errors.New("agent is disabled")
	}
	return s.providerCache.get(cfg.LLM)
}

// Save writes the agent configuration to the JSON file.
// Uses atomic write (temp file + rename) to prevent corruption.
func (s *Store) Save(_ context.Context, cfg *agent.Config) error {
	if cfg == nil {
		return errors.New("fileagentconfig: config cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.writeConfigToFile(s.configPath(), cfg)
}

// writeConfigToFile writes the config to a JSON file atomically.
func (s *Store) writeConfigToFile(filePath string, cfg *agent.Config) error {
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
func applyEnvOverrides(cfg *agent.Config) {
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
