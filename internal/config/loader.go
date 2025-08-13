package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/adrg/xdg"
	"github.com/dagu-org/dagu/internal/build"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/spf13/viper"
)

// UsedConfigFile is a global variable that stores the path to the configuration file
var UsedConfigFile = atomic.Value{}

// loadLock synchronizes access to the Load function to ensure that only one configuration load occurs at a time.
var loadLock sync.Mutex

// Load creates a new configuration by instantiating a ConfigLoader with the provided options
// and then invoking its Load method.
func Load(opts ...ConfigLoaderOption) (*Config, error) {
	loadLock.Lock()
	defer loadLock.Unlock()

	loader := NewConfigLoader(opts...)
	cfg, err := loader.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return cfg, nil
}

// ConfigLoader is responsible for reading and merging configuration from various sources.
// The internal mutex ensures thread-safety when loading the configuration.
type ConfigLoader struct {
	lock       sync.Mutex
	configFile string   // Optional explicit path to the configuration file.
	warnings   []string // Collected warnings during configuration resolution.
}

// ConfigLoaderOption defines a functional option for configuring a ConfigLoader.
type ConfigLoaderOption func(*ConfigLoader)

// WithConfigFile returns a ConfigLoaderOption that sets the configuration file path.
func WithConfigFile(configFile string) ConfigLoaderOption {
	return func(l *ConfigLoader) {
		l.configFile = configFile
	}
}

// NewConfigLoader creates a new ConfigLoader instance and applies all given options.
func NewConfigLoader(options ...ConfigLoaderOption) *ConfigLoader {
	loader := &ConfigLoader{}
	for _, option := range options {
		option(loader)
	}
	return loader
}

// Load initializes viper, reads configuration files, handles legacy configuration,
// and returns a fully built and validated Config instance.
func (l *ConfigLoader) Load() (*Config, error) {
	l.lock.Lock()
	defer l.lock.Unlock()

	// Initialize viper with proper defaults, environment binding and warnings.
	if err := l.setupViper(); err != nil {
		return nil, fmt.Errorf("viper setup failed: %w", err)
	}

	// Attempt to read the main config file. If not found, we proceed without error.
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}
	configPath := viper.ConfigFileUsed()

	// Store the path of the used configuration file for later reference.
	if configFile := viper.ConfigFileUsed(); configFile != "" {
		UsedConfigFile.Store(configFile)
	}

	// For backward compatibility, try merging in the "admin.yaml" config.
	viper.SetConfigName("admin")
	if err := viper.MergeInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read admin config: %w", err)
		}
	}

	// Unmarshal the merged configuration into our Definition structure.
	var def Definition
	if err := viper.Unmarshal(&def); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Build the final Config from the definition (including legacy fields and validations).
	cfg, err := l.buildConfig(def)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}

	// Attach any warnings collected during the resolution process.
	cfg.Warnings = l.warnings

	// Set the config path in the global configuration for reference.
	cfg.Global.ConfigPath = configPath

	return cfg, nil
}

// buildConfig transforms the intermediate Definition (raw config data) into a final Config structure.
// It also handles legacy fields, environment variable overrides, and validations.
func (l *ConfigLoader) buildConfig(def Definition) (*Config, error) {
	var cfg Config

	// Set global configuration values.
	cfg.Global = Global{
		Debug:        def.Debug,
		LogFormat:    def.LogFormat,
		TZ:           def.TZ,
		WorkDir:      def.WorkDir,
		DefaultShell: def.DefaultShell,
		SkipExamples: viper.GetBool("skipExamples"),
	}

	// Set Peer configuration if provided
	if def.Peer.CertFile != "" || def.Peer.KeyFile != "" || def.Peer.ClientCaFile != "" || def.Peer.SkipTLSVerify || def.Peer.Insecure {
		cfg.Global.Peer = Peer{
			CertFile:      def.Peer.CertFile,
			KeyFile:       def.Peer.KeyFile,
			ClientCaFile:  def.Peer.ClientCaFile,
			SkipTLSVerify: def.Peer.SkipTLSVerify,
			Insecure:      def.Peer.Insecure,
		}
	}

	// Initialize the timezone (loads the time.Location and sets the TZ environment variable).
	if err := cfg.Global.setTimezone(); err != nil {
		return nil, fmt.Errorf("failed to set timezone: %w", err)
	}

	// Populate server configuration.
	cfg.Server = Server{
		Host:        def.Host,
		Port:        def.Port,
		BasePath:    def.BasePath,
		APIBasePath: def.APIBasePath,
		Permissions: map[Permission]bool{
			PermissionWriteDAGs: true,
			PermissionRunDAGs:   true,
		},
	}

	// Permissions can be nil, so we check before dereferencing.
	if def.Permissions.WriteDAGs != nil {
		cfg.Server.Permissions[PermissionWriteDAGs] = *def.Permissions.WriteDAGs
	}
	if def.PermissionWriteDAGs != nil {
		cfg.Server.Permissions[PermissionWriteDAGs] = *def.PermissionWriteDAGs
	}
	if def.Permissions.RunDAGs != nil {
		cfg.Server.Permissions[PermissionRunDAGs] = *def.Permissions.RunDAGs
	}
	if def.PermissionRunDAGs != nil {
		cfg.Server.Permissions[PermissionRunDAGs] = *def.PermissionRunDAGs
	}

	// Process remote node definitions.
	for _, node := range def.RemoteNodes {
		cfg.Server.RemoteNodes = append(cfg.Server.RemoteNodes, RemoteNode(node))
	}

	// Dereference pointer fields if they are provided.
	if def.Headless != nil {
		cfg.Server.Headless = *def.Headless
	}
	if def.LatestStatusToday != nil {
		cfg.Server.LatestStatusToday = *def.LatestStatusToday
	}

	// Set TLS configuration if available.
	if def.TLS != nil {
		cfg.Server.TLS = &TLSConfig{
			CertFile: def.TLS.CertFile,
			KeyFile:  def.TLS.KeyFile,
			CAFile:   def.TLS.CAFile,
		}
	}

	// Process authentication settings.
	if def.Auth != nil {
		if def.Auth.Basic != nil {
			cfg.Server.Auth.Basic.Username = def.Auth.Basic.Username
			cfg.Server.Auth.Basic.Password = def.Auth.Basic.Password
		}
		if def.Auth.Token != nil {
			cfg.Server.Auth.Token.Value = def.Auth.Token.Value
		}
		if def.Auth.OIDC != nil {
			cfg.Server.Auth.OIDC.ClientId = def.Auth.OIDC.ClientId
			cfg.Server.Auth.OIDC.ClientSecret = def.Auth.OIDC.ClientSecret
			cfg.Server.Auth.OIDC.ClientUrl = def.Auth.OIDC.ClientUrl
			cfg.Server.Auth.OIDC.Issuer = def.Auth.OIDC.Issuer
			cfg.Server.Auth.OIDC.Scopes = def.Auth.OIDC.Scopes
			cfg.Server.Auth.OIDC.Whitelist = def.Auth.OIDC.Whitelist
		}
	}

	// Normalize the BasePath value for proper URL construction.
	cfg.Server.cleanBasePath()

	// Set file system paths from the definition.
	if def.Paths != nil {
		cfg.Paths.DAGsDir = fileutil.ResolvePathOrBlank(def.Paths.DAGsDir)
		cfg.Paths.SuspendFlagsDir = fileutil.ResolvePathOrBlank(def.Paths.SuspendFlagsDir)
		cfg.Paths.DataDir = fileutil.ResolvePathOrBlank(def.Paths.DataDir)
		cfg.Paths.LogDir = fileutil.ResolvePathOrBlank(def.Paths.LogDir)
		cfg.Paths.AdminLogsDir = fileutil.ResolvePathOrBlank(def.Paths.AdminLogsDir)
		cfg.Paths.BaseConfig = fileutil.ResolvePathOrBlank(def.Paths.BaseConfig)
		cfg.Paths.Executable = fileutil.ResolvePathOrBlank(def.Paths.Executable)
		cfg.Paths.DAGRunsDir = fileutil.ResolvePathOrBlank(def.Paths.DAGRunsDir)
		cfg.Paths.QueueDir = fileutil.ResolvePathOrBlank(def.Paths.QueueDir)
		cfg.Paths.ProcDir = fileutil.ResolvePathOrBlank(def.Paths.ProcDir)
		cfg.Paths.ServiceRegistryDir = fileutil.ResolvePathOrBlank(def.Paths.ServiceRegistryDir)
	}

	// Set UI configuration if provided.
	if def.UI != nil {
		cfg.UI.NavbarColor = def.UI.NavbarColor
		cfg.UI.NavbarTitle = def.UI.NavbarTitle
		cfg.UI.MaxDashboardPageLimit = def.UI.MaxDashboardPageLimit
		cfg.UI.LogEncodingCharset = def.UI.LogEncodingCharset

		// Set DAGs configuration if provided
		if def.UI.DAGs != nil {
			cfg.UI.DAGs.SortField = def.UI.DAGs.SortField
			cfg.UI.DAGs.SortOrder = def.UI.DAGs.SortOrder
		}
	}

	// Set queue configuration if provided, with default enabled=true.
	cfg.Queues.Enabled = true // Default to enabled
	if def.Queues != nil {
		cfg.Queues.Enabled = def.Queues.Enabled
		for _, queueDef := range def.Queues.Config {
			cfg.Queues.Config = append(cfg.Queues.Config, QueueConfig{
				Name:          queueDef.Name,
				MaxActiveRuns: queueDef.MaxActiveRuns,
			})
		}
	}

	// Override with values from config file if provided
	if def.Coordinator != nil {
		cfg.Coordinator.Host = def.Coordinator.Host
		cfg.Coordinator.Port = def.Coordinator.Port
	}

	// Set worker configuration from nested structure
	if def.Worker != nil {
		cfg.Worker.ID = def.Worker.ID
		cfg.Worker.MaxActiveRuns = def.Worker.MaxActiveRuns

		// Parse worker labels - can be either string or map
		if def.Worker.Labels != nil {
			switch v := def.Worker.Labels.(type) {
			case string:
				if v != "" {
					cfg.Worker.Labels = parseLabels(v)
				}
			case map[string]interface{}:
				cfg.Worker.Labels = make(map[string]string)
				for key, val := range v {
					if strVal, ok := val.(string); ok {
						cfg.Worker.Labels[key] = strVal
					}
				}
			case map[interface{}]interface{}:
				cfg.Worker.Labels = make(map[string]string)
				for key, val := range v {
					if keyStr, ok := key.(string); ok {
						if valStr, ok := val.(string); ok {
							cfg.Worker.Labels[keyStr] = valStr
						}
					}
				}
			}
		}
	}

	// Set scheduler configuration
	if def.Scheduler != nil {
		cfg.Scheduler.Port = def.Scheduler.Port

		// Parse scheduler lock stale threshold
		if def.Scheduler.LockStaleThreshold != "" {
			if duration, err := time.ParseDuration(def.Scheduler.LockStaleThreshold); err == nil {
				cfg.Scheduler.LockStaleThreshold = duration
			} else {
				l.warnings = append(l.warnings, fmt.Sprintf("Invalid scheduler.lockStaleThreshold value: %s", def.Scheduler.LockStaleThreshold))
			}
		}

		// Parse scheduler lock retry interval
		if def.Scheduler.LockRetryInterval != "" {
			if duration, err := time.ParseDuration(def.Scheduler.LockRetryInterval); err == nil {
				cfg.Scheduler.LockRetryInterval = duration
			} else {
				l.warnings = append(l.warnings, fmt.Sprintf("Invalid scheduler.lockRetryInterval value: %s", def.Scheduler.LockRetryInterval))
			}
		}

		// Parse scheduler zombie detection interval
		if def.Scheduler.ZombieDetectionInterval != "" {
			if duration, err := time.ParseDuration(def.Scheduler.ZombieDetectionInterval); err == nil {
				cfg.Scheduler.ZombieDetectionInterval = duration
			} else {
				l.warnings = append(l.warnings, fmt.Sprintf("Invalid scheduler.zombieDetectionInterval value: %s", def.Scheduler.ZombieDetectionInterval))
			}
		}
	}

	if cfg.Scheduler.Port <= 0 {
		cfg.Scheduler.Port = 8090 // Default scheduler port
	}
	if cfg.Scheduler.LockStaleThreshold <= 0 {
		cfg.Scheduler.LockStaleThreshold = 30 * time.Second // Default to 30 seconds if not set
	}
	if cfg.Scheduler.LockRetryInterval <= 0 {
		cfg.Scheduler.LockRetryInterval = 5 * time.Second // Default to 5 seconds if not set
	}
	// Only set default if not explicitly configured (including env vars)
	// Check if the value is still zero after parsing, which means it wasn't set
	if cfg.Scheduler.ZombieDetectionInterval == 0 && !viper.IsSet("scheduler.zombieDetectionInterval") {
		cfg.Scheduler.ZombieDetectionInterval = 45 * time.Second // Default to 45 seconds if not set
	}

	// Incorporate legacy field values, which may override existing settings.
	l.LoadLegacyFields(&cfg, def)

	// Load legacy environment variable overrides.
	l.LoadLegacyEnv(&cfg)

	// Setup the directory inside the datadir.
	if cfg.Paths.DAGRunsDir == "" {
		cfg.Paths.DAGRunsDir = filepath.Join(cfg.Paths.DataDir, "dag-runs")
	}
	if cfg.Paths.ProcDir == "" {
		cfg.Paths.ProcDir = filepath.Join(cfg.Paths.DataDir, "proc")
	}
	if cfg.Paths.QueueDir == "" {
		cfg.Paths.QueueDir = filepath.Join(cfg.Paths.DataDir, "queue")
	}
	if cfg.Paths.ServiceRegistryDir == "" {
		cfg.Paths.ServiceRegistryDir = filepath.Join(cfg.Paths.DataDir, "service-registry")
	}

	// Ensure the executable path is set.
	if err := l.setExecutable(&cfg); err != nil {
		return nil, fmt.Errorf("failed to set executable: %w", err)
	}

	// Validate the final configuration.
	if err := l.validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// LoadLegacyFields copies values from legacy configuration fields into the current Config structure.
// Legacy fields are only applied if they are non-empty or non-zero, and may override the new settings.
func (l *ConfigLoader) LoadLegacyFields(cfg *Config, def Definition) {
	if def.BasicAuthUsername != "" {
		cfg.Server.Auth.Basic.Username = def.BasicAuthUsername
	}
	if def.BasicAuthPassword != "" {
		cfg.Server.Auth.Basic.Password = def.BasicAuthPassword
	}
	if def.APIBaseURL != "" {
		cfg.Server.APIBasePath = def.APIBaseURL
	}
	if def.IsAuthToken {
		cfg.Server.Auth.Token.Value = def.AuthToken
	}
	// For DAGs directory, if both legacy fields are present, def.DAGsDir takes precedence.
	if def.DAGs != "" {
		cfg.Paths.DAGsDir = fileutil.ResolvePathOrBlank(def.DAGs)
	}
	if def.DAGsDir != "" {
		cfg.Paths.DAGsDir = fileutil.ResolvePathOrBlank(def.DAGsDir)
	}
	if def.Executable != "" {
		cfg.Paths.Executable = fileutil.ResolvePathOrBlank(def.Executable)
	}
	if def.LogDir != "" {
		cfg.Paths.LogDir = fileutil.ResolvePathOrBlank(def.LogDir)
	}
	if def.DataDir != "" {
		cfg.Paths.DataDir = fileutil.ResolvePathOrBlank(def.DataDir)
	}
	if def.SuspendFlagsDir != "" {
		cfg.Paths.SuspendFlagsDir = fileutil.ResolvePathOrBlank(def.SuspendFlagsDir)
	}
	if def.AdminLogsDir != "" {
		cfg.Paths.AdminLogsDir = fileutil.ResolvePathOrBlank(def.AdminLogsDir)
	}
	if def.BaseConfig != "" {
		cfg.Paths.BaseConfig = fileutil.ResolvePathOrBlank(def.BaseConfig)
	}
	if def.LogEncodingCharset != "" {
		cfg.UI.LogEncodingCharset = def.LogEncodingCharset
	}
	if def.NavbarColor != "" {
		cfg.UI.NavbarColor = def.NavbarColor
	}
	if def.NavbarTitle != "" {
		cfg.UI.NavbarTitle = def.NavbarTitle
	}
	if def.MaxDashboardPageLimit > 0 {
		cfg.UI.MaxDashboardPageLimit = def.MaxDashboardPageLimit
	}
}

// setupViper initializes viper by determining the home directory and XDG configuration,
// configuring viper with defaults, binding environment variables, and collecting any warnings.
func (l *ConfigLoader) setupViper() error {
	homeDir, err := l.getHomeDir()
	if err != nil {
		return err
	}
	xdgConfig := l.getXDGConfig(homeDir)
	resolver := NewResolver("DAGU_HOME", filepath.Join(homeDir, ".dagu"), xdgConfig)

	// Collect any warnings from path resolution.
	l.warnings = append(l.warnings, resolver.Warnings...)

	l.configureViper(resolver)
	l.bindEnvironmentVariables()
	l.setDefaultValues(resolver)

	return nil
}

// getHomeDir returns the current user's home directory.
func (l *ConfigLoader) getHomeDir() (string, error) {
	dir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return dir, nil
}

// getXDGConfig creates an XDGConfig using the provided home directory.
func (l *ConfigLoader) getXDGConfig(homeDir string) XDGConfig {
	return XDGConfig{
		DataHome:   xdg.DataHome,
		ConfigHome: filepath.Join(homeDir, ".config"),
	}
}

// configureViper sets up viper's configuration file location, type, and environment variable handling.
func (l *ConfigLoader) configureViper(resolver PathResolver) {
	l.setupViperConfigPath(resolver.ConfigDir)
	viper.SetEnvPrefix(strings.ToUpper(build.Slug))
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
}

func (l *ConfigLoader) setupViperConfigPath(configDir string) {
	if l.configFile == "" {
		viper.AddConfigPath(configDir)
		viper.SetConfigName("config")
	} else {
		viper.SetConfigFile(l.configFile)
	}
	viper.SetConfigType("yaml")
}

// setDefaultValues establishes the default configuration values for various keys.
func (l *ConfigLoader) setDefaultValues(resolver PathResolver) {
	// File paths
	viper.SetDefault("workDir", "") // Defaults to DAG location if empty.
	viper.SetDefault("skipExamples", false) // Defaults to creating examples
	viper.SetDefault("paths.dagsDir", resolver.DAGsDir)
	viper.SetDefault("paths.suspendFlagsDir", resolver.SuspendFlagsDir)
	viper.SetDefault("paths.dataDir", resolver.DataDir)
	viper.SetDefault("paths.logDir", resolver.LogsDir)
	viper.SetDefault("paths.adminLogsDir", resolver.AdminLogsDir)
	viper.SetDefault("paths.baseConfig", resolver.BaseConfigFile)

	// Server settings
	viper.SetDefault("host", "127.0.0.1")
	viper.SetDefault("port", 8080)
	viper.SetDefault("debug", false)
	viper.SetDefault("basePath", "")
	viper.SetDefault("apiBasePath", "/api/v2")
	viper.SetDefault("latestStatusToday", false)

	// Coordinator settings
	viper.SetDefault("coordinatorHost", "127.0.0.1")
	viper.SetDefault("coordinatorPort", 50055)

	// Worker settings - nested structure
	viper.SetDefault("worker.maxActiveRuns", 100)

	// UI settings
	viper.SetDefault("ui.navbarTitle", build.AppName)
	viper.SetDefault("ui.maxDashboardPageLimit", 100)
	viper.SetDefault("ui.logEncodingCharset", "utf-8")
	viper.SetDefault("ui.dags.sortField", "name")
	viper.SetDefault("ui.dags.sortOrder", "asc")

	// Logging settings
	viper.SetDefault("logFormat", "text")

	// Queue settings
	viper.SetDefault("queues.enabled", true)

	// Scheduler settings
	viper.SetDefault("scheduler.lockStaleThreshold", "30s")
	viper.SetDefault("scheduler.lockRetryInterval", "5s")

	// Peer settings
	viper.SetDefault("peer.insecure", true) // Default to insecure (h2c)
}

// bindEnvironmentVariables binds various configuration keys to environment variables.
func (l *ConfigLoader) bindEnvironmentVariables() {
	// Server configurations
	l.bindEnv("logFormat", "LOG_FORMAT")
	l.bindEnv("basePath", "BASE_PATH")
	l.bindEnv("apiBaseURL", "API_BASE_URL")
	l.bindEnv("tz", "TZ")
	l.bindEnv("host", "HOST")
	l.bindEnv("port", "PORT")
	l.bindEnv("debug", "DEBUG")
	l.bindEnv("headless", "HEADLESS")

	// Global configurations
	l.bindEnv("workDir", "WORK_DIR")
	l.bindEnv("defaultShell", "DEFAULT_SHELL")
	l.bindEnv("skipExamples", "SKIP_EXAMPLES")

	// Scheduler configurations
	l.bindEnv("scheduler.lockStaleThreshold", "SCHEDULER_LOCK_STALE_THRESHOLD")
	l.bindEnv("scheduler.lockRetryInterval", "SCHEDULER_LOCK_RETRY_INTERVAL")
	l.bindEnv("scheduler.zombieDetectionInterval", "SCHEDULER_ZOMBIE_DETECTION_INTERVAL")

	// UI configurations
	l.bindEnv("ui.maxDashboardPageLimit", "UI_MAX_DASHBOARD_PAGE_LIMIT")
	l.bindEnv("ui.logEncodingCharset", "UI_LOG_ENCODING_CHARSET")
	l.bindEnv("ui.navbarColor", "UI_NAVBAR_COLOR")
	l.bindEnv("ui.navbarTitle", "UI_NAVBAR_TITLE")
	l.bindEnv("ui.dags.sortField", "UI_DAGS_SORT_FIELD")
	l.bindEnv("ui.dags.sortOrder", "UI_DAGS_SORT_ORDER")

	// UI configurations (legacy keys)
	l.bindEnv("ui.maxDashboardPageLimit", "MAX_DASHBOARD_PAGE_LIMIT")
	l.bindEnv("ui.logEncodingCharset", "LOG_ENCODING_CHARSET")
	l.bindEnv("ui.navbarColor", "NAVBAR_COLOR")
	l.bindEnv("ui.navbarTitle", "NAVBAR_TITLE")

	// Authentication configurations
	l.bindEnv("auth.basic.username", "AUTH_BASIC_USERNAME")
	l.bindEnv("auth.basic.password", "AUTH_BASIC_PASSWORD")
	l.bindEnv("auth.token.value", "AUTH_TOKEN")

	// Authentication configurations (OIDC)
	l.bindEnv("auth.oidc.clientId", "AUTH_OIDC_CLIENT_ID")
	l.bindEnv("auth.oidc.clientSecret", "AUTH_OIDC_CLIENT_SECRET")
	l.bindEnv("auth.oidc.clientUrl", "AUTH_OIDC_CLIENT_URL")
	l.bindEnv("auth.oidc.issuer", "AUTH_OIDC_ISSUER")
	l.bindEnv("auth.oidc.scopes", "AUTH_OIDC_SCOPES")
	l.bindEnv("auth.oidc.whitelist", "AUTH_OIDC_WHITELIST")

	// Authentication configurations (legacy keys)
	l.bindEnv("auth.basic.username", "BASICAUTH_USERNAME")
	l.bindEnv("auth.basic.password", "BASICAUTH_PASSWORD")
	l.bindEnv("auth.token.value", "AUTHTOKEN")

	// TLS configurations
	l.bindEnv("tls.certFile", "CERT_FILE")
	l.bindEnv("tls.keyFile", "KEY_FILE")

	// File paths
	l.bindEnv("paths.dagsDir", "DAGS")
	l.bindEnv("paths.dagsDir", "DAGS_DIR")
	l.bindEnv("paths.executable", "EXECUTABLE")
	l.bindEnv("paths.logDir", "LOG_DIR")
	l.bindEnv("paths.dataDir", "DATA_DIR")
	l.bindEnv("paths.suspendFlagsDir", "SUSPEND_FLAGS_DIR")
	l.bindEnv("paths.adminLogsDir", "ADMIN_LOG_DIR")
	l.bindEnv("paths.baseConfig", "BASE_CONFIG")
	l.bindEnv("paths.dagRunsDir", "DAG_RUNS_DIR")
	l.bindEnv("paths.procDir", "PROC_DIR")
	l.bindEnv("paths.queueDir", "QUEUE_DIR")
	l.bindEnv("paths.serviceRegistryDir", "SERVICE_REGISTRY_DIR")

	// UI customization
	l.bindEnv("latestStatusToday", "LATEST_STATUS_TODAY")

	// Queue configuration
	l.bindEnv("queues.enabled", "QUEUE_ENABLED")

	// Coordinator service configuration (flat structure)
	l.bindEnv("coordinator.host", "COORDINATOR_HOST")
	l.bindEnv("coordinator.port", "COORDINATOR_PORT")

	// Worker configuration (nested structure)
	l.bindEnv("worker.id", "WORKER_ID")
	l.bindEnv("worker.maxActiveRuns", "WORKER_MAX_ACTIVE_RUNS")
	l.bindEnv("worker.labels", "WORKER_LABELS")
	// Scheduler configuration
	l.bindEnv("scheduler.port", "SCHEDULER_PORT")

	// Peer configuration
	l.bindEnv("peer.certFile", "PEER_CERT_FILE")
	l.bindEnv("peer.keyFile", "PEER_KEY_FILE")
	l.bindEnv("peer.clientCaFile", "PEER_CLIENT_CA_FILE")
	l.bindEnv("peer.skipTlsVerify", "PEER_SKIP_TLS_VERIFY")
	l.bindEnv("peer.insecure", "PEER_INSECURE")
}

// bindEnv constructs the full environment variable name using the app prefix and binds it to the given key.
func (l *ConfigLoader) bindEnv(key, env string) {
	prefix := strings.ToUpper(build.Slug) + "_"
	_ = viper.BindEnv(key, prefix+env)
}

// LoadLegacyEnv maps legacy environment variables to their new counterparts in the configuration.
// If a legacy env var is set, a warning is logged and the corresponding setter function is called.
func (l *ConfigLoader) LoadLegacyEnv(cfg *Config) {
	legacyEnvs := map[string]struct {
		newKey string
		setter func(*Config, string)
	}{
		"DAGU__ADMIN_NAVBAR_COLOR": {
			newKey: "DAGU_NAVBAR_COLOR",
			setter: func(c *Config, v string) { c.UI.NavbarColor = v },
		},
		"DAGU__ADMIN_NAVBAR_TITLE": {
			newKey: "DAGU_NAVBAR_TITLE",
			setter: func(c *Config, v string) { c.UI.NavbarTitle = v },
		},
		"DAGU__ADMIN_PORT": {
			newKey: "DAGU_PORT",
			setter: func(c *Config, v string) {
				if i, err := strconv.Atoi(v); err == nil {
					c.Server.Port = i
				}
			},
		},
		"DAGU__ADMIN_HOST": {
			newKey: "DAGU_HOST",
			setter: func(c *Config, v string) { c.Server.Host = v },
		},
		"DAGU__DATA": {
			newKey: "DAGU_DATA_DIR",
			setter: func(c *Config, v string) { c.Paths.DataDir = v },
		},
		"DAGU__SUSPEND_FLAGS_DIR": {
			newKey: "DAGU_SUSPEND_FLAGS_DIR",
			setter: func(c *Config, v string) { c.Paths.SuspendFlagsDir = v },
		},
		"DAGU__ADMIN_LOGS_DIR": {
			newKey: "DAGU_ADMIN_LOG_DIR",
			setter: func(c *Config, v string) { c.Paths.AdminLogsDir = v },
		},
	}

	// For each legacy variable, if it is set, log a warning and update the configuration.
	for oldKey, mapping := range legacyEnvs {
		if value := os.Getenv(oldKey); value != "" {
			log.Printf("%s is deprecated. Use %s instead.", oldKey, mapping.newKey)
			mapping.setter(cfg, value)
		}
	}
}

// setExecutable ensures that the executable path is set in the configuration.
// If not provided, it retrieves the current executable's path.
func (l *ConfigLoader) setExecutable(cfg *Config) error {
	if cfg.Paths.Executable == "" {
		executable, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}
		cfg.Paths.Executable = executable
	}
	return nil
}

// validateConfig performs basic validation on the configuration to ensure required fields are set
// and that numerical values fall within acceptable ranges.
func (l *ConfigLoader) validateConfig(cfg *Config) error {
	if cfg.Server.Port < 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", cfg.Server.Port)
	}

	if cfg.Server.TLS != nil {
		if cfg.Server.TLS.CertFile == "" || cfg.Server.TLS.KeyFile == "" {
			return fmt.Errorf("TLS configuration incomplete: both cert and key files are required")
		}
	}

	// Redundant check for port validity (can be removed if not needed twice).
	if cfg.Server.Port < 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", cfg.Server.Port)
	}

	if cfg.UI.MaxDashboardPageLimit < 1 {
		return fmt.Errorf("invalid max dashboard page limit: %d", cfg.UI.MaxDashboardPageLimit)
	}

	return nil
}

// parseLabels parses a comma-separated string of key=value pairs into a map.
// Example: "gpu=true,memory=64G" -> map[string]string{"gpu": "true", "memory": "64G"}
func parseLabels(labelsStr string) map[string]string {
	labels := make(map[string]string)
	if labelsStr == "" {
		return labels
	}

	pairs := strings.Split(labelsStr, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if key != "" {
				labels[key] = value
			}
		}
	}

	return labels
}
