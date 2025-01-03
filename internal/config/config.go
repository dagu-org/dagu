package config

import (
	"fmt"
	"os"
	"path"
	"sync"
	"time"
)

// Config represents the server configuration with both new and legacy fields
type Config struct {
	// Server settings
	Host        string `mapstructure:"host"`
	Port        int    `mapstructure:"port"`
	Debug       bool   `mapstructure:"debug"`
	BasePath    string `mapstructure:"basePath"`
	APIBasePath string `mapstructure:"apiBasePath"`
	APIBaseURL  string `mapstructure:"apiBaseURL"` // For backward compatibility
	WorkDir     string `mapstructure:"workDir"`

	// Authentication
	Auth Auth `mapstructure:"auth"`

	// File system paths
	Paths PathsConfig `mapstructure:"paths"`

	// Legacy fields for backward compatibility - Start
	// Note: These fields are used for backward compatibility and should not be used in new code
	// Deprecated: Use Auth.Basic.Enabled instead
	DAGs string `mapstructure:"dags"`
	// Deprecated: Use Paths.Executable instead
	Executable string `mapstructure:"executable"`
	// Deprecated: Use Paths.LogDir instead
	LogDir string `mapstructure:"logDir"`
	// Deprecated: Use Paths.DataDir instead
	DataDir string `mapstructure:"dataDir"`
	// Deprecated: Use Paths.SuspendFlagsDir instead
	SuspendFlagsDir string `mapstructure:"suspendFlagsDir"`
	// Deprecated: Use Paths.AdminLogsDir instead
	AdminLogsDir string `mapstructure:"adminLogsDir"`
	// Deprecated: Use Paths.BaseConfig instead
	BaseConfig string `mapstructure:"baseConfig"`
	// Deprecated: Use Auth.Token.Enabled instead
	IsBasicAuth bool `mapstructure:"isBasicAuth"`
	// Deprecated: Use Auth.Basic.Username instead
	BasicAuthUsername string `mapstructure:"basicAuthUsername"`
	// Deprecated: Use Auth.Basic.Password instead
	BasicAuthPassword string `mapstructure:"basicAuthPassword"`
	// Deprecated: Use Auth.Token.Enabled instead
	IsAuthToken bool `mapstructure:"isAuthToken"`
	// Deprecated: Use Auth.Token.Value instead
	AuthToken string `mapstructure:"authToken"`
	// Deprecated: Use UI.LogEncodingCharset instead
	LogEncodingCharset string `mapstructure:"logEncodingCharset"`
	// Deprecated: Use UI.NavbarColor instead
	NavbarColor string `mapstructure:"navbarColor"`
	// Deprecated: Use UI.NavbarTitle instead
	NavbarTitle string `mapstructure:"navbarTitle"`
	// Deprecated: Use UI.MaxDashboardPageLimit instead
	MaxDashboardPageLimit int `mapstructure:"maxDashboardPageLimit"`
	// Legacy fields for backward compatibility - End

	// Other settings
	LogFormat         string         `mapstructure:"logFormat"`
	LatestStatusToday bool           `mapstructure:"latestStatusToday"`
	TZ                string         `mapstructure:"tz"`
	Location          *time.Location `mapstructure:"-"`
	Env               sync.Map       `mapstructure:"-"`

	UI UI `mapstructure:"ui"`

	// Remote nodes configuration
	RemoteNodes []RemoteNode `mapstructure:"remoteNodes"`

	// TLS configuration
	TLS *TLSConfig `mapstructure:"tls"`
}

// Auth represents the authentication configuration
type Auth struct {
	Basic AuthBasic `mapstructure:"basic"`
	Token AuthToken `mapstructure:"token"`
}

// AuthBasic represents the basic authentication configuration
type AuthBasic struct {
	Enabled  bool   `mapstructure:"enabled"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// AuthToken represents the authentication token configuration
type AuthToken struct {
	Enabled bool   `mapstructure:"enabled"`
	Value   string `mapstructure:"value"`
}

// Paths represents the file system paths configuration
type PathsConfig struct {
	DAGsDir         string `mapstructure:"dagsDir"`
	Executable      string `mapstructure:"executable"`
	LogDir          string `mapstructure:"logDir"`
	DataDir         string `mapstructure:"dataDir"`
	SuspendFlagsDir string `mapstructure:"suspendFlagsDir"`
	AdminLogsDir    string `mapstructure:"adminLogsDir"`
	BaseConfig      string `mapstructure:"baseConfig"`
}

type UI struct {
	LogEncodingCharset    string `mapstructure:"logEncodingCharset"`
	NavbarColor           string `mapstructure:"navbarColor"`
	NavbarTitle           string `mapstructure:"navbarTitle"`
	MaxDashboardPageLimit int    `mapstructure:"maxDashboardPageLimit"`
}

// RemoteNode represents a remote node configuration
type RemoteNode struct {
	Name              string `mapstructure:"name"`
	APIBaseURL        string `mapstructure:"apiBaseURL"`
	IsBasicAuth       bool   `mapstructure:"isBasicAuth"`
	BasicAuthUsername string `mapstructure:"basicAuthUsername"`
	BasicAuthPassword string `mapstructure:"basicAuthPassword"`
	IsAuthToken       bool   `mapstructure:"isAuthToken"`
	AuthToken         string `mapstructure:"authToken"`
	SkipTLSVerify     bool   `mapstructure:"skipTLSVerify"`
}

// TLSConfig represents TLS configuration
type TLSConfig struct {
	CertFile string `mapstructure:"certFile"`
	KeyFile  string `mapstructure:"keyFile"`
}

// MigrateLegacyConfig migrates legacy configuration
func (c *Config) MigrateLegacyConfig() {
	// Migrate server settings
	c.migrateServerSettings()

	// Migrate authentication settings
	c.migrateAuthSettings()

	// Migrate paths
	c.migratePaths()

	// Migrate UI settings
	c.migrateUISettings()

	// Clean base path
	c.cleanBasePath()
}

func (c *Config) migrateServerSettings() {
	if c.APIBaseURL != "" {
		c.APIBasePath = c.APIBaseURL
	}
}

func (c *Config) migrateAuthSettings() {
	if c.IsBasicAuth {
		c.Auth.Basic.Enabled = c.IsBasicAuth
		c.Auth.Basic.Username = c.BasicAuthUsername
		c.Auth.Basic.Password = c.BasicAuthPassword
	}

	if c.IsAuthToken {
		c.Auth.Token.Enabled = c.IsAuthToken
		c.Auth.Token.Value = c.AuthToken
	}
}

func (c *Config) migratePaths() {
	if c.DAGs != "" {
		c.Paths.DAGsDir = c.DAGs
	}
	if c.Executable != "" {
		c.Paths.Executable = c.Executable
	}
	if c.LogDir != "" {
		c.Paths.LogDir = c.LogDir
	}
	if c.DataDir != "" {
		c.Paths.DataDir = c.DataDir
	}
	if c.SuspendFlagsDir != "" {
		c.Paths.SuspendFlagsDir = c.SuspendFlagsDir
	}
	if c.AdminLogsDir != "" {
		c.Paths.AdminLogsDir = c.AdminLogsDir
	}
	if c.BaseConfig != "" {
		c.Paths.BaseConfig = c.BaseConfig
	}
}

func (c *Config) migrateUISettings() {
	if c.LogEncodingCharset != "" {
		c.UI.LogEncodingCharset = c.LogEncodingCharset
	}
	if c.NavbarColor != "" {
		c.UI.NavbarColor = c.NavbarColor
	}
	if c.NavbarTitle != "" {
		c.UI.NavbarTitle = c.NavbarTitle
	}
	if c.MaxDashboardPageLimit > 0 {
		c.UI.MaxDashboardPageLimit = c.MaxDashboardPageLimit
	}
}

func (c *Config) cleanBasePath() {
	if c.BasePath != "" {
		c.BasePath = path.Clean(c.BasePath)
		if !path.IsAbs(c.BasePath) {
			c.BasePath = path.Join("/", c.BasePath)
		}
		if c.BasePath == "/" {
			c.BasePath = ""
		}
	}
}

// Load creates a new configuration with backward compatibility
func Load() (*Config, error) {
	loader := NewConfigLoader()
	cfg, err := loader.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Load legacy environment variables
	if err := loader.LoadLegacyEnv(cfg); err != nil {
		return nil, fmt.Errorf("failed to load legacy env: %w", err)
	}

	// Migrate legacy configuration
	cfg.MigrateLegacyConfig()

	// Set environment variables
	cfg.setEnvVariables()

	return cfg, nil
}

func (c *Config) setEnvVariables() {
	c.Env.Range(func(k, v any) bool {
		key := k.(string)
		value := v.(string)
		if err := os.Setenv(key, value); err != nil {
			fmt.Printf("failed to set env variable %s: %v\n", key, err)
		}
		return true
	})
}
