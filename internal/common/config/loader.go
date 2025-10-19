package config

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adrg/xdg"
	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/spf13/viper"
)

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
		return nil, err
	}
	return cfg, nil
}

// ConfigLoader is responsible for reading and merging configuration from various sources.
// The internal mutex ensures thread-safety when loading the configuration.
type ConfigLoader struct {
	lock              sync.Mutex
	configFile        string   // Optional explicit path to the configuration file.
	warnings          []string // Collected warnings during configuration resolution.
	additionalBaseEnv []string // Additional environment variables to append to the base environment.
	appHomeDir        string   // Optional override for DAGU_HOME style directory.
}

// ConfigLoaderOption defines a functional option for configuring a ConfigLoader.
type ConfigLoaderOption func(*ConfigLoader)

// WithConfigFile returns a ConfigLoaderOption that sets the configuration file path.
func WithConfigFile(configFile string) ConfigLoaderOption {
	return func(l *ConfigLoader) {
		l.configFile = configFile
	}
}

// WithAppHomeDir sets a custom application home directory (equivalent to DAGU_HOME).
func WithAppHomeDir(dir string) ConfigLoaderOption {
	return func(l *ConfigLoader) {
		l.appHomeDir = dir
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
	homeDir, err := getHomeDir()
	if err != nil {
		return nil, err
	}
	xdgConfig := XDGConfig{
		DataHome:   xdg.DataHome,
		ConfigHome: filepath.Join(homeDir, ".config"),
	}
	if l.appHomeDir != "" {
		l.additionalBaseEnv = append(l.additionalBaseEnv, fmt.Sprintf("DAGU_HOME=%s", fileutil.ResolvePathOrBlank(l.appHomeDir)))
	}
	warnings := setupViper(xdgConfig, homeDir, l.configFile, l.appHomeDir)
	l.warnings = append(l.warnings, warnings...)

	// Attempt to read the main config file. If not found, we proceed without error.
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
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

	cfg.Global.ConfigFileUsed = viper.ConfigFileUsed()

	// Attach any warnings collected during the resolution process.
	cfg.Warnings = l.warnings

	return cfg, nil
}

// buildConfig transforms the intermediate Definition (raw config data) into a final Config structure.
// It also handles legacy fields, environment variable overrides, and validations.
func (l *ConfigLoader) buildConfig(def Definition) (*Config, error) {
	var cfg Config

	baseEnv := LoadBaseEnv()
	baseEnv.variables = append(baseEnv.variables, l.additionalBaseEnv...)

	// Set global configuration values.
	cfg.Global = Global{
		Debug:        def.Debug,
		LogFormat:    def.LogFormat,
		TZ:           def.TZ,
		DefaultShell: def.DefaultShell,
		SkipExamples: viper.GetBool("skipExamples"),
		BaseEnv:      baseEnv,
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
	if err := setTimezone(&cfg.Global); err != nil {
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
	cfg.Server.BasePath = cleanServerBasePath(cfg.Server.BasePath)

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
			queueConfig := QueueConfig{
				Name:          queueDef.Name,
				MaxActiveRuns: queueDef.MaxConcurrency,
			}
			// For backward compatibility
			if queueDef.MaxActiveRuns != nil {
				queueConfig.MaxActiveRuns = *queueDef.MaxActiveRuns
			}
			cfg.Queues.Config = append(cfg.Queues.Config, queueConfig)
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
					cfg.Worker.Labels = parseWorkerLabels(v)
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
	if cfg.Paths.Executable == "" {
		executable, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("failed to get executable path: %w", err)
		}
		cfg.Paths.Executable = executable
	}

	// Validate the final configuration.
	if err := cfg.Validate(); err != nil {
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

func setupViper(xdgConfig XDGConfig, homeDir, configFile, appHomeOverride string) (warnings []string) {
	var paths Paths
	if appHomeOverride != "" {
		resolved := fileutil.ResolvePathOrBlank(appHomeOverride)
		paths = setUnifiedPaths(resolved)
	} else {
		paths = ResolvePaths("DAGU_HOME", filepath.Join(homeDir, ".dagu"), xdgConfig)
	}

	configureViper(paths.ConfigDir, configFile)
	bindEnvironmentVariables()
	setViperDefaultValues(paths)

	return paths.Warnings
}

func setViperDefaultValues(paths Paths) {
	// File paths
	viper.SetDefault("workDir", "")         // Defaults to DAG location if empty.
	viper.SetDefault("skipExamples", false) // Defaults to creating examples
	viper.SetDefault("paths.dagsDir", paths.DAGsDir)
	viper.SetDefault("paths.suspendFlagsDir", paths.SuspendFlagsDir)
	viper.SetDefault("paths.dataDir", paths.DataDir)
	viper.SetDefault("paths.logDir", paths.LogsDir)
	viper.SetDefault("paths.adminLogsDir", paths.AdminLogsDir)
	viper.SetDefault("paths.baseConfig", paths.BaseConfigFile)

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
	viper.SetDefault("ui.navbarTitle", AppName)
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
func bindEnvironmentVariables() {
	// Server configurations
	bindEnv("logFormat", "LOG_FORMAT")
	bindEnv("basePath", "BASE_PATH")
	bindEnv("apiBaseURL", "API_BASE_URL")
	bindEnv("tz", "TZ")
	bindEnv("host", "HOST")
	bindEnv("port", "PORT")
	bindEnv("debug", "DEBUG")
	bindEnv("headless", "HEADLESS")

	// Global configurations
	bindEnv("workDir", "WORK_DIR")
	bindEnv("defaultShell", "DEFAULT_SHELL")
	bindEnv("skipExamples", "SKIP_EXAMPLES")

	// Scheduler configurations
	bindEnv("scheduler.lockStaleThreshold", "SCHEDULER_LOCK_STALE_THRESHOLD")
	bindEnv("scheduler.lockRetryInterval", "SCHEDULER_LOCK_RETRY_INTERVAL")
	bindEnv("scheduler.zombieDetectionInterval", "SCHEDULER_ZOMBIE_DETECTION_INTERVAL")

	// UI configurations
	bindEnv("ui.maxDashboardPageLimit", "UI_MAX_DASHBOARD_PAGE_LIMIT")
	bindEnv("ui.logEncodingCharset", "UI_LOG_ENCODING_CHARSET")
	bindEnv("ui.navbarColor", "UI_NAVBAR_COLOR")
	bindEnv("ui.navbarTitle", "UI_NAVBAR_TITLE")
	bindEnv("ui.dags.sortField", "UI_DAGS_SORT_FIELD")
	bindEnv("ui.dags.sortOrder", "UI_DAGS_SORT_ORDER")

	// UI configurations (legacy keys)
	bindEnv("ui.maxDashboardPageLimit", "MAX_DASHBOARD_PAGE_LIMIT")
	bindEnv("ui.logEncodingCharset", "LOG_ENCODING_CHARSET")
	bindEnv("ui.navbarColor", "NAVBAR_COLOR")
	bindEnv("ui.navbarTitle", "NAVBAR_TITLE")

	// Authentication configurations
	bindEnv("auth.basic.username", "AUTH_BASIC_USERNAME")
	bindEnv("auth.basic.password", "AUTH_BASIC_PASSWORD")
	bindEnv("auth.token.value", "AUTH_TOKEN")

	// Authentication configurations (OIDC)
	bindEnv("auth.oidc.clientId", "AUTH_OIDC_CLIENT_ID")
	bindEnv("auth.oidc.clientSecret", "AUTH_OIDC_CLIENT_SECRET")
	bindEnv("auth.oidc.clientUrl", "AUTH_OIDC_CLIENT_URL")
	bindEnv("auth.oidc.issuer", "AUTH_OIDC_ISSUER")
	bindEnv("auth.oidc.scopes", "AUTH_OIDC_SCOPES")
	bindEnv("auth.oidc.whitelist", "AUTH_OIDC_WHITELIST")

	// Authentication configurations (legacy keys)
	bindEnv("auth.basic.username", "BASICAUTH_USERNAME")
	bindEnv("auth.basic.password", "BASICAUTH_PASSWORD")
	bindEnv("auth.token.value", "AUTHTOKEN")

	// TLS configurations
	bindEnv("tls.certFile", "CERT_FILE")
	bindEnv("tls.keyFile", "KEY_FILE")

	// File paths
	bindEnv("paths.dagsDir", "DAGS")
	bindEnv("paths.dagsDir", "DAGS_DIR")
	bindEnv("paths.executable", "EXECUTABLE")
	bindEnv("paths.logDir", "LOG_DIR")
	bindEnv("paths.dataDir", "DATA_DIR")
	bindEnv("paths.suspendFlagsDir", "SUSPEND_FLAGS_DIR")
	bindEnv("paths.adminLogsDir", "ADMIN_LOG_DIR")
	bindEnv("paths.baseConfig", "BASE_CONFIG")
	bindEnv("paths.dagRunsDir", "DAG_RUNS_DIR")
	bindEnv("paths.procDir", "PROC_DIR")
	bindEnv("paths.queueDir", "QUEUE_DIR")
	bindEnv("paths.serviceRegistryDir", "SERVICE_REGISTRY_DIR")

	// UI customization
	bindEnv("latestStatusToday", "LATEST_STATUS_TODAY")

	// Queue configuration
	bindEnv("queues.enabled", "QUEUE_ENABLED")

	// Coordinator service configuration (flat structure)
	bindEnv("coordinator.host", "COORDINATOR_HOST")
	bindEnv("coordinator.port", "COORDINATOR_PORT")

	// Worker configuration (nested structure)
	bindEnv("worker.id", "WORKER_ID")
	bindEnv("worker.maxActiveRuns", "WORKER_MAX_ACTIVE_RUNS")
	bindEnv("worker.labels", "WORKER_LABELS")
	// Scheduler configuration
	bindEnv("scheduler.port", "SCHEDULER_PORT")

	// Peer configuration
	bindEnv("peer.certFile", "PEER_CERT_FILE")
	bindEnv("peer.keyFile", "PEER_KEY_FILE")
	bindEnv("peer.clientCaFile", "PEER_CLIENT_CA_FILE")
	bindEnv("peer.skipTlsVerify", "PEER_SKIP_TLS_VERIFY")
	bindEnv("peer.insecure", "PEER_INSECURE")
}

func bindEnv(key, env string) {
	prefix := strings.ToUpper(AppSlug) + "_"
	_ = viper.BindEnv(key, prefix+env)
}

func configureViper(configDir, configFile string) {
	if configFile == "" {
		viper.AddConfigPath(configDir)
		viper.SetConfigName("config")
	} else {
		viper.SetConfigFile(configFile)
	}
	viper.SetConfigType("yaml")
	viper.SetEnvPrefix(strings.ToUpper(AppSlug))
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
}

// validateConfig performs basic validation on the configuration to ensure required fields are set
// and that numerical values fall within acceptable ranges.
func validateConfig(cfg *Config) error {
	if cfg.Server.Port < 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", cfg.Server.Port)
	}

	if cfg.Server.TLS != nil {
		if cfg.Server.TLS.CertFile == "" || cfg.Server.TLS.KeyFile == "" {
			return fmt.Errorf("TLS configuration incomplete: both cert and key files are required")
		}
	}

	if cfg.UI.MaxDashboardPageLimit < 1 {
		return fmt.Errorf("invalid max dashboard page limit: %d", cfg.UI.MaxDashboardPageLimit)
	}

	return nil
}

func parseWorkerLabels(labelsStr string) map[string]string {
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

func cleanServerBasePath(s string) string {
	if s == "" {
		return ""
	}

	// Clean the provided BasePath.
	cleanPath := path.Clean(s)

	// Ensure the path is absolute.
	if !path.IsAbs(cleanPath) {
		cleanPath = path.Join("/", cleanPath)
	}

	if cleanPath == "/" {
		// If the cleaned path is the root, reset it to an empty string.
		return ""
	}
	return cleanPath
}

func getHomeDir() (string, error) {
	dir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return dir, nil
}
