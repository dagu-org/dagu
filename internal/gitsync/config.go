package gitsync

import (
	"strconv"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/core/exec"
)

// Config holds the configuration for Git sync functionality.
type Config struct {
	// Enabled indicates whether Git sync is enabled.
	Enabled bool

	// Repository is the Git repository URL.
	// Format: github.com/org/repo or https://github.com/org/repo.git
	Repository string

	// Branch is the branch to sync with.
	Branch string

	// Path is the subdirectory within the repository to sync.
	// Empty string means root directory.
	Path string

	// Auth contains authentication configuration.
	Auth AuthConfig

	// AutoSync contains auto-sync configuration.
	AutoSync AutoSyncConfig

	// PushEnabled indicates whether pushing changes is allowed.
	PushEnabled bool

	// Commit contains commit configuration.
	Commit CommitConfig
}

// AuthConfig holds authentication configuration for Git operations.
type AuthConfig struct {
	// Type is the authentication type: "token" or "ssh".
	Type string

	// Token is the personal access token for HTTPS authentication.
	Token string

	// SSHKeyPath is the path to the SSH private key file.
	SSHKeyPath string

	// SSHPassphrase is the passphrase for the SSH key (optional).
	SSHPassphrase string
}

// AutoSyncConfig holds configuration for automatic synchronization.
type AutoSyncConfig struct {
	// Enabled indicates whether auto-sync is enabled.
	Enabled bool

	// OnStartup indicates whether to sync on server startup.
	OnStartup bool

	// Interval is the sync interval in seconds.
	// 0 means auto-sync is disabled (pull on startup only).
	Interval int
}

// CommitConfig holds configuration for Git commits.
type CommitConfig struct {
	// AuthorName is the name to use for commits.
	// Defaults to "Dagu" if not specified.
	AuthorName string

	// AuthorEmail is the email to use for commits.
	// Defaults to "dagu@localhost" if not specified.
	AuthorEmail string
}

// AuthType constants for authentication types.
const (
	AuthTypeToken = "token"
	AuthTypeSSH   = "ssh"
)

// IsValid returns true if the configuration is valid for sync operations.
func (c *Config) IsValid() bool {
	return c.Enabled && c.Repository != "" && c.Branch != ""
}

// GetAuthorName returns the commit author name, using default if not set.
func (c *Config) GetAuthorName() string {
	if c.Commit.AuthorName != "" {
		return c.Commit.AuthorName
	}
	return "Dagu"
}

// GetAuthorEmail returns the commit author email, using default if not set.
func (c *Config) GetAuthorEmail() string {
	if c.Commit.AuthorEmail != "" {
		return c.Commit.AuthorEmail
	}
	return "dagu@localhost"
}

// NewConfigFromNamespace creates a gitsync.Config from a namespace's git sync settings.
// The SSHKeyRef field from NamespaceGitSync is mapped to Auth.SSHKeyPath.
func NewConfigFromNamespace(ns exec.NamespaceGitSync) *Config {
	enabled := ns.RemoteURL != ""
	cfg := &Config{
		Enabled:    enabled,
		Repository: ns.RemoteURL,
		Branch:     ns.Branch,
		Path:       ns.Path,
	}
	if ns.SSHKeyRef != "" {
		cfg.Auth = AuthConfig{
			Type:       AuthTypeSSH,
			SSHKeyPath: ns.SSHKeyRef,
		}
	}
	if ns.AutoSyncInterval != "" {
		if seconds := parseIntervalSeconds(ns.AutoSyncInterval); seconds > 0 {
			cfg.AutoSync = AutoSyncConfig{
				Enabled:  true,
				Interval: seconds,
			}
		}
	}
	return cfg
}

// parseIntervalSeconds parses a duration string (e.g. "5m", "1h", "30s") into seconds.
func parseIntervalSeconds(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Try Go duration format first (e.g. "5m", "1h30m", "30s")
	d, err := time.ParseDuration(s)
	if err == nil {
		return int(d.Seconds())
	}
	// Fall back to plain integer (seconds)
	n, err := strconv.Atoi(s)
	if err == nil {
		return n
	}
	return 0
}

// NewConfigFromGlobal creates a gitsync.Config from the global configuration.
func NewConfigFromGlobal(cfg config.GitSyncConfig) *Config {
	return &Config{
		Enabled:     cfg.Enabled,
		Repository:  cfg.Repository,
		Branch:      cfg.Branch,
		Path:        cfg.Path,
		PushEnabled: cfg.PushEnabled,
		Auth: AuthConfig{
			Type:          cfg.Auth.Type,
			Token:         cfg.Auth.Token,
			SSHKeyPath:    cfg.Auth.SSHKeyPath,
			SSHPassphrase: cfg.Auth.SSHPassphrase,
		},
		AutoSync: AutoSyncConfig{
			Enabled:   cfg.AutoSync.Enabled,
			OnStartup: cfg.AutoSync.OnStartup,
			Interval:  cfg.AutoSync.Interval,
		},
		Commit: CommitConfig{
			AuthorName:  cfg.Commit.AuthorName,
			AuthorEmail: cfg.Commit.AuthorEmail,
		},
	}
}
