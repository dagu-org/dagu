package config

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/spf13/viper"
)

// Service represents the type of service that is loading configuration.
// This allows the loader to only load configuration sections relevant to
// the specific service, improving performance and reducing unnecessary processing.
type Service int

const (
	// ServiceNone indicates no specific service - loads all configuration.
	// Use this for CLI commands or when full configuration is needed.
	ServiceNone Service = iota

	// ServiceServer is the web UI server service.
	// Requires: Core, Server, Paths, UI, Queues, Coordinator, Monitoring
	ServiceServer

	// ServiceScheduler is the scheduler service for automated DAG execution.
	// Requires: Core, Paths, Scheduler, Queues, Coordinator
	ServiceScheduler

	// ServiceWorker is the worker service that polls the coordinator for tasks.
	// Requires: Core, Paths, Worker, Coordinator (peer config)
	ServiceWorker

	// ServiceCoordinator is the coordinator gRPC server for distributed execution.
	// Requires: Core, Paths, Coordinator
	ServiceCoordinator

	// ServiceAgent is for the agent executor (runs DAGs).
	// Requires: Core, Paths, Queues (to check if distributed execution is enabled)
	ServiceAgent
)

// ConfigLoader is responsible for reading and merging configuration from various sources.
type ConfigLoader struct {
	v                 *viper.Viper // Isolated viper instance for this loader
	configFile        string       // Optional explicit path to the configuration file.
	warnings          []string     // Collected warnings during configuration resolution.
	additionalBaseEnv []string     // Additional environment variables to append to the base environment.
	appHomeDir        string       // Optional override for DAGU_HOME style directory.
	service           Service      // The service type being configured (determines which config sections to load).
}

// ConfigLoaderOption defines a functional option for configuring a ConfigLoader.
type ConfigLoaderOption func(*ConfigLoader)

// WithConfigFile returns a ConfigLoaderOption that sets the configuration file path.
func WithConfigFile(configFile string) ConfigLoaderOption {
	return func(l *ConfigLoader) {
		l.configFile = configFile
	}
}

// WithAppHomeDir returns a ConfigLoaderOption that sets the application home directory
// used by the ConfigLoader, overriding the default DAGU_HOME resolution.
func WithAppHomeDir(dir string) ConfigLoaderOption {
	return func(l *ConfigLoader) {
		l.appHomeDir = dir
	}
}

// WithService sets the service type for configuration loading.
// This allows the loader to only process configuration sections
// ConfigLoader to determine which configuration sections to load.
func WithService(service Service) ConfigLoaderOption {
	return func(l *ConfigLoader) {
		l.service = service
	}
}

// ConfigSection represents a specific section of the configuration using bit flags.
type ConfigSection uint16

const (
	SectionNone        ConfigSection = 0
	SectionServer      ConfigSection = 1 << iota // 2
	SectionScheduler                             // 4
	SectionWorker                                // 8
	SectionCoordinator                           // 16
	SectionUI                                    // 32
	SectionQueues                                // 64
	SectionMonitoring                            // 128

	// SectionAll combines all sections (useful for ServiceNone/CLI)
	SectionAll = SectionServer | SectionScheduler | SectionWorker | SectionCoordinator | SectionUI | SectionQueues | SectionMonitoring
)

// serviceRequirements maps services to their required config sections using bitwise OR.
var serviceRequirements = map[Service]ConfigSection{
	ServiceNone:        SectionAll,
	ServiceServer:      SectionServer | SectionCoordinator | SectionUI | SectionQueues | SectionMonitoring,
	ServiceScheduler:   SectionScheduler | SectionCoordinator | SectionQueues,
	ServiceWorker:      SectionWorker | SectionCoordinator,
	ServiceCoordinator: SectionCoordinator,
	ServiceAgent:       SectionQueues,
}

// requires checks if the loader's service requires the given config section.
func (l *ConfigLoader) requires(section ConfigSection) bool {
	req, ok := serviceRequirements[l.service]
	if !ok {
		req = SectionAll
	}
	return req&section != 0
}

// resolvePath resolves a path to an absolute path with a descriptive error message.
// Returns the resolved path, or an error if resolution fails.
// Empty paths are returned as-is without error.
func (l *ConfigLoader) resolvePath(fieldName, path string) (string, error) {
	if path == "" {
		return "", nil
	}
	resolved, err := fileutil.ResolvePath(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve %s path %q: %w", fieldName, path, err)
	}
	return resolved, nil
}

// NewConfigLoader creates a new ConfigLoader instance with an isolated viper instance
// ConfigLoaderOption values (for example: service, config file path, or app home directory).
func NewConfigLoader(viper *viper.Viper, options ...ConfigLoaderOption) *ConfigLoader {
	loader := &ConfigLoader{v: viper}
	for _, option := range options {
		option(loader)
	}
	return loader
}

// Load initializes viper, reads configuration files, handles legacy configuration,
// and returns a fully built and validated Config instance.
func (l *ConfigLoader) Load() (*Config, error) {
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
	warnings, err := l.setupViper(xdgConfig, homeDir, l.configFile, l.appHomeDir)
	if err != nil {
		return nil, err
	}
	l.warnings = append(l.warnings, warnings...)

	// Attempt to read the main config file. If not found, we proceed without error.
	if err := l.v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}
	configFileUsed, err := l.resolvePath("config file", l.v.ConfigFileUsed())
	if err != nil {
		return nil, err
	}

	// For backward compatibility, try merging in the "admin.yaml" config.
	l.v.SetConfigName("admin")
	if err := l.v.MergeInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read admin config: %w", err)
		}
	}

	// Unmarshal the merged configuration into our Definition structure.
	var def Definition
	if err := l.v.Unmarshal(&def); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Build the final Config from the definition (including legacy fields and validations).
	cfg, err := l.buildConfig(def)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}

	cfg.Paths.ConfigFileUsed = configFileUsed

	// Attach any warnings collected during the resolution process.
	cfg.Warnings = l.warnings

	return cfg, nil
}

// buildConfig transforms the intermediate Definition (raw config data) into a final Config structure.
// It uses the service requirements to load only necessary configuration sections.
func (l *ConfigLoader) buildConfig(def Definition) (*Config, error) {
	var cfg Config

	// Always load core and paths configuration
	if err := l.loadCoreConfig(&cfg, def); err != nil {
		return nil, err
	}
	if err := l.loadPathsConfig(&cfg, def); err != nil {
		return nil, err
	}

	// Load service-specific configuration based on requirements
	if l.requires(SectionServer) {
		l.loadServerConfig(&cfg, def)
	}
	if l.requires(SectionUI) {
		l.loadUIConfig(&cfg, def)
	}
	if l.requires(SectionQueues) {
		l.loadQueuesConfig(&cfg, def)
	}
	if l.requires(SectionCoordinator) {
		l.loadCoordinatorConfig(&cfg, def)
	}
	if l.requires(SectionWorker) {
		l.loadWorkerConfig(&cfg, def)
	}
	if l.requires(SectionScheduler) {
		l.loadSchedulerConfig(&cfg, def)
	}
	if l.requires(SectionMonitoring) {
		l.loadMonitoringConfig(&cfg, def)
	}

	// Cache config is always loaded (used by all services)
	l.loadCacheConfig(&cfg, def)

	// Incorporate legacy field values and environment variable overrides
	if err := l.LoadLegacyFields(&cfg, def); err != nil {
		return nil, err
	}
	l.loadLegacyEnv(&cfg)

	// Finalize paths (set derived paths based on DataDir)
	l.finalizePaths(&cfg)

	// Validate configuration
	if l.requires(SectionServer) {
		if err := l.validateServerConfig(&cfg); err != nil {
			return nil, err
		}
	}
	if l.requires(SectionUI) {
		if err := l.validateUIConfig(&cfg); err != nil {
			return nil, err
		}
	}

	return &cfg, nil
}

// loadCoreConfig loads the core configuration settings.
func (l *ConfigLoader) loadCoreConfig(cfg *Config, def Definition) error {
	baseEnv := LoadBaseEnv()
	baseEnv.variables = append(baseEnv.variables, l.additionalBaseEnv...)

	cfg.Core = Core{
		Debug:        def.Debug,
		LogFormat:    def.LogFormat,
		TZ:           def.TZ,
		DefaultShell: def.DefaultShell,
		SkipExamples: l.v.GetBool("skipExamples"),
		BaseEnv:      baseEnv,
	}

	// Set Peer configuration if provided
	if def.Peer.CertFile != "" || def.Peer.KeyFile != "" || def.Peer.ClientCaFile != "" || def.Peer.SkipTLSVerify || def.Peer.Insecure {
		cfg.Core.Peer = Peer{
			CertFile:      def.Peer.CertFile,
			KeyFile:       def.Peer.KeyFile,
			ClientCaFile:  def.Peer.ClientCaFile,
			SkipTLSVerify: def.Peer.SkipTLSVerify,
			Insecure:      def.Peer.Insecure,
		}
	}

	// Initialize the timezone
	if err := setTimezone(&cfg.Core); err != nil {
		return fmt.Errorf("failed to set timezone: %w", err)
	}

	return nil
}

// loadPathsConfig loads the file system paths configuration.
// All paths are resolved to absolute paths. Returns an error if any path resolution fails.
func (l *ConfigLoader) loadPathsConfig(cfg *Config, def Definition) error {
	if def.Paths == nil {
		return nil
	}

	var err error
	if cfg.Paths.DAGsDir, err = l.resolvePath("DAGsDir", def.Paths.DAGsDir); err != nil {
		return err
	}
	if cfg.Paths.SuspendFlagsDir, err = l.resolvePath("SuspendFlagsDir", def.Paths.SuspendFlagsDir); err != nil {
		return err
	}
	if cfg.Paths.DataDir, err = l.resolvePath("DataDir", def.Paths.DataDir); err != nil {
		return err
	}
	if cfg.Paths.LogDir, err = l.resolvePath("LogDir", def.Paths.LogDir); err != nil {
		return err
	}
	if cfg.Paths.AdminLogsDir, err = l.resolvePath("AdminLogsDir", def.Paths.AdminLogsDir); err != nil {
		return err
	}
	if cfg.Paths.BaseConfig, err = l.resolvePath("BaseConfig", def.Paths.BaseConfig); err != nil {
		return err
	}
	if cfg.Paths.Executable, err = l.resolvePath("Executable", def.Paths.Executable); err != nil {
		return err
	}
	if cfg.Paths.DAGRunsDir, err = l.resolvePath("DAGRunsDir", def.Paths.DAGRunsDir); err != nil {
		return err
	}
	if cfg.Paths.QueueDir, err = l.resolvePath("QueueDir", def.Paths.QueueDir); err != nil {
		return err
	}
	if cfg.Paths.ProcDir, err = l.resolvePath("ProcDir", def.Paths.ProcDir); err != nil {
		return err
	}
	if cfg.Paths.ServiceRegistryDir, err = l.resolvePath("ServiceRegistryDir", def.Paths.ServiceRegistryDir); err != nil {
		return err
	}
	if cfg.Paths.UsersDir, err = l.resolvePath("UsersDir", def.Paths.UsersDir); err != nil {
		return err
	}
	if cfg.Paths.APIKeysDir, err = l.resolvePath("APIKeysDir", def.Paths.APIKeysDir); err != nil {
		return err
	}
	if cfg.Paths.WebhooksDir, err = l.resolvePath("WebhooksDir", def.Paths.WebhooksDir); err != nil {
		return err
	}

	return nil
}

// loadServerConfig loads the server configuration.
func (l *ConfigLoader) loadServerConfig(cfg *Config, def Definition) {
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
	var explicitAuthMode bool
	if def.Auth != nil && def.Auth.Mode != nil {
		mode := AuthMode(*def.Auth.Mode)
		switch mode {
		case AuthModeNone, AuthModeBuiltin, AuthModeOIDC:
			cfg.Server.Auth.Mode = mode
			explicitAuthMode = true
		default:
			l.warnings = append(l.warnings, fmt.Sprintf("Invalid auth.mode value: %q, defaulting to 'none'", *def.Auth.Mode))
			cfg.Server.Auth.Mode = AuthModeNone
		}
	} else {
		cfg.Server.Auth.Mode = AuthModeNone
	}

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
		if def.Auth.Builtin != nil {
			if def.Auth.Builtin.Admin != nil {
				cfg.Server.Auth.Builtin.Admin.Username = def.Auth.Builtin.Admin.Username
				cfg.Server.Auth.Builtin.Admin.Password = def.Auth.Builtin.Admin.Password
			}
			if def.Auth.Builtin.Token != nil {
				cfg.Server.Auth.Builtin.Token.Secret = def.Auth.Builtin.Token.Secret
				if def.Auth.Builtin.Token.TTL != "" {
					if duration, err := time.ParseDuration(def.Auth.Builtin.Token.TTL); err == nil {
						cfg.Server.Auth.Builtin.Token.TTL = duration
					} else {
						l.warnings = append(l.warnings, fmt.Sprintf("Invalid auth.builtin.token.ttl value: %s", def.Auth.Builtin.Token.TTL))
					}
				}
			}
		}

		// Auto-detect auth mode if not explicitly set
		if !explicitAuthMode {
			oidc := cfg.Server.Auth.OIDC
			if oidc.ClientId != "" && oidc.ClientSecret != "" && oidc.Issuer != "" {
				cfg.Server.Auth.Mode = AuthModeOIDC
				l.warnings = append(l.warnings, fmt.Sprintf("Auth mode auto-detected as 'oidc' based on OIDC configuration (issuer: %s)", oidc.Issuer))
			}
		}

		// Warn if basic auth is configured with builtin auth mode (it will be ignored)
		if cfg.Server.Auth.Mode == AuthModeBuiltin {
			if cfg.Server.Auth.Basic.Username != "" || cfg.Server.Auth.Basic.Password != "" {
				l.warnings = append(l.warnings, "Basic auth configuration is ignored when auth mode is 'builtin'; use builtin auth's admin credentials instead")
			}
		}
	}

	// Set default token TTL if not specified
	if cfg.Server.Auth.Builtin.Token.TTL <= 0 {
		cfg.Server.Auth.Builtin.Token.TTL = 24 * time.Hour
	}
	// Set default admin username if not specified
	if cfg.Server.Auth.Builtin.Admin.Username == "" {
		cfg.Server.Auth.Builtin.Admin.Username = "admin"
	}

	// Normalize the BasePath value for proper URL construction.
	cfg.Server.BasePath = cleanServerBasePath(cfg.Server.BasePath)

	// Set metrics access mode (default: private)
	cfg.Server.Metrics = MetricsAccessPrivate
	if def.Metrics != nil {
		switch MetricsAccess(*def.Metrics) {
		case MetricsAccessPublic, MetricsAccessPrivate:
			cfg.Server.Metrics = MetricsAccess(*def.Metrics)
		default:
			l.warnings = append(l.warnings, fmt.Sprintf("Invalid server.metrics value: %q, defaulting to 'private'", *def.Metrics))
		}
	}
}

// validateServerConfig validates the server configuration.
func (l *ConfigLoader) validateServerConfig(cfg *Config) error {
	if cfg.Server.Port < 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", cfg.Server.Port)
	}
	if cfg.Server.TLS != nil {
		if cfg.Server.TLS.CertFile == "" || cfg.Server.TLS.KeyFile == "" {
			return fmt.Errorf("TLS configuration incomplete: both cert and key files are required")
		}
	}
	return cfg.validateBuiltinAuth()
}

// loadUIConfig loads the UI configuration.
func (l *ConfigLoader) loadUIConfig(cfg *Config, def Definition) {
	// Apply defaults from viper (these include the configured defaults)
	cfg.UI.MaxDashboardPageLimit = l.v.GetInt("ui.maxDashboardPageLimit")
	cfg.UI.NavbarTitle = l.v.GetString("ui.navbarTitle")
	cfg.UI.LogEncodingCharset = l.v.GetString("ui.logEncodingCharset")
	cfg.UI.DAGs.SortField = l.v.GetString("ui.dags.sortField")
	cfg.UI.DAGs.SortOrder = l.v.GetString("ui.dags.sortOrder")

	if def.UI != nil {
		cfg.UI.NavbarColor = def.UI.NavbarColor
		if def.UI.NavbarTitle != "" {
			cfg.UI.NavbarTitle = def.UI.NavbarTitle
		}
		if def.UI.MaxDashboardPageLimit > 0 {
			cfg.UI.MaxDashboardPageLimit = def.UI.MaxDashboardPageLimit
		}
		if def.UI.LogEncodingCharset != "" {
			cfg.UI.LogEncodingCharset = def.UI.LogEncodingCharset
		}

		if def.UI.DAGs != nil {
			if def.UI.DAGs.SortField != "" {
				cfg.UI.DAGs.SortField = def.UI.DAGs.SortField
			}
			if def.UI.DAGs.SortOrder != "" {
				cfg.UI.DAGs.SortOrder = def.UI.DAGs.SortOrder
			}
		}
	}
}

// validateUIConfig validates the UI configuration.
func (l *ConfigLoader) validateUIConfig(cfg *Config) error {
	if cfg.UI.MaxDashboardPageLimit < 1 {
		return fmt.Errorf("invalid max dashboard page limit: %d", cfg.UI.MaxDashboardPageLimit)
	}
	return nil
}

// loadQueuesConfig loads the queue configuration.
func (l *ConfigLoader) loadQueuesConfig(cfg *Config, def Definition) {
	cfg.Queues.Enabled = true // Default to enabled
	if def.Queues != nil {
		cfg.Queues.Enabled = def.Queues.Enabled
		for _, queueDef := range def.Queues.Config {
			queueConfig := QueueConfig{
				Name:          queueDef.Name,
				MaxActiveRuns: queueDef.MaxConcurrency,
			}
			if queueDef.MaxActiveRuns != nil {
				queueConfig.MaxActiveRuns = *queueDef.MaxActiveRuns
			}
			cfg.Queues.Config = append(cfg.Queues.Config, queueConfig)
		}
	}
}

// loadCoordinatorConfig loads the coordinator configuration.
func (l *ConfigLoader) loadCoordinatorConfig(cfg *Config, def Definition) {
	if def.Coordinator != nil {
		cfg.Coordinator.Host = def.Coordinator.Host
		cfg.Coordinator.Advertise = def.Coordinator.Advertise
		cfg.Coordinator.Port = def.Coordinator.Port
	}
}

// loadWorkerConfig loads the worker configuration.
func (l *ConfigLoader) loadWorkerConfig(cfg *Config, def Definition) {
	if def.Worker != nil {
		cfg.Worker.ID = def.Worker.ID
		cfg.Worker.MaxActiveRuns = def.Worker.MaxActiveRuns

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
}

// loadSchedulerConfig loads the scheduler configuration.
func (l *ConfigLoader) loadSchedulerConfig(cfg *Config, def Definition) {
	if def.Scheduler != nil {
		cfg.Scheduler.Port = def.Scheduler.Port

		if def.Scheduler.LockStaleThreshold != "" {
			if duration, err := time.ParseDuration(def.Scheduler.LockStaleThreshold); err == nil {
				cfg.Scheduler.LockStaleThreshold = duration
			} else {
				l.warnings = append(l.warnings, fmt.Sprintf("Invalid scheduler.lockStaleThreshold value: %s", def.Scheduler.LockStaleThreshold))
			}
		}

		if def.Scheduler.LockRetryInterval != "" {
			if duration, err := time.ParseDuration(def.Scheduler.LockRetryInterval); err == nil {
				cfg.Scheduler.LockRetryInterval = duration
			} else {
				l.warnings = append(l.warnings, fmt.Sprintf("Invalid scheduler.lockRetryInterval value: %s", def.Scheduler.LockRetryInterval))
			}
		}

		if def.Scheduler.ZombieDetectionInterval != "" {
			if duration, err := time.ParseDuration(def.Scheduler.ZombieDetectionInterval); err == nil {
				cfg.Scheduler.ZombieDetectionInterval = duration
			} else {
				l.warnings = append(l.warnings, fmt.Sprintf("Invalid scheduler.zombieDetectionInterval value: %s", def.Scheduler.ZombieDetectionInterval))
			}
		}
	}

	// Set defaults
	if cfg.Scheduler.Port <= 0 {
		cfg.Scheduler.Port = 8090
	}
	if cfg.Scheduler.LockStaleThreshold <= 0 {
		cfg.Scheduler.LockStaleThreshold = 30 * time.Second
	}
	if cfg.Scheduler.LockRetryInterval <= 0 {
		cfg.Scheduler.LockRetryInterval = 5 * time.Second
	}
	// Default ZombieDetectionInterval only if not explicitly set.
	// An invalid value logs a warning but falls back to 0 (disabled).
	// Setting to 0 or negative disables zombie detection.
	if cfg.Scheduler.ZombieDetectionInterval <= 0 && !l.v.IsSet("scheduler.zombieDetectionInterval") {
		cfg.Scheduler.ZombieDetectionInterval = 45 * time.Second
	}
}

// loadMonitoringConfig loads the monitoring configuration.
func (l *ConfigLoader) loadMonitoringConfig(cfg *Config, def Definition) {
	if def.Monitoring != nil {
		if def.Monitoring.Retention != "" {
			if duration, err := time.ParseDuration(def.Monitoring.Retention); err == nil {
				cfg.Monitoring.Retention = duration
			} else {
				l.warnings = append(l.warnings, fmt.Sprintf("Invalid monitoring.retention value: %s", def.Monitoring.Retention))
			}
		}
		if def.Monitoring.Interval != "" {
			if duration, err := time.ParseDuration(def.Monitoring.Interval); err == nil {
				cfg.Monitoring.Interval = duration
			} else {
				l.warnings = append(l.warnings, fmt.Sprintf("Invalid monitoring.interval value: %s", def.Monitoring.Interval))
			}
		}
	}

	// Set defaults
	if cfg.Monitoring.Retention <= 0 {
		cfg.Monitoring.Retention = 24 * time.Hour
	}
	if cfg.Monitoring.Interval <= 0 {
		cfg.Monitoring.Interval = 5 * time.Second
	}
}

// loadCacheConfig loads the cache configuration.
func (l *ConfigLoader) loadCacheConfig(cfg *Config, def Definition) {
	if def.Cache != nil {
		mode := CacheMode(*def.Cache)
		if mode.IsValid() {
			cfg.Cache = mode
		} else {
			l.warnings = append(l.warnings, fmt.Sprintf("Invalid cache mode: %q, using 'normal'", *def.Cache))
			cfg.Cache = CacheModeNormal
		}
	} else {
		cfg.Cache = CacheModeNormal
	}
}

// finalizePaths sets up derived paths and ensures required paths are set.
func (l *ConfigLoader) finalizePaths(cfg *Config) {
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
	if cfg.Paths.UsersDir == "" {
		cfg.Paths.UsersDir = filepath.Join(cfg.Paths.DataDir, "users")
	}
	if cfg.Paths.APIKeysDir == "" {
		cfg.Paths.APIKeysDir = filepath.Join(cfg.Paths.DataDir, "apikeys")
	}
	if cfg.Paths.WebhooksDir == "" {
		cfg.Paths.WebhooksDir = filepath.Join(cfg.Paths.DataDir, "webhooks")
	}

	if cfg.Paths.Executable == "" {
		if executable, err := os.Executable(); err == nil {
			cfg.Paths.Executable = executable
		}
	}
}

// LoadLegacyFields copies values from legacy configuration fields into the current Config structure.
// Legacy fields are only applied if they are non-empty or non-zero, and may override the new settings.
// It respects the service requirements to only apply relevant legacy fields.
// Returns an error if any path resolution fails.
func (l *ConfigLoader) LoadLegacyFields(cfg *Config, def Definition) error {
	// Server-related legacy fields
	if l.requires(SectionServer) {
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
	}

	// Path-related legacy fields (always applied)
	var err error
	if def.DAGs != "" {
		if cfg.Paths.DAGsDir, err = l.resolvePath("legacy DAGs", def.DAGs); err != nil {
			return err
		}
	}
	if def.DAGsDir != "" {
		if cfg.Paths.DAGsDir, err = l.resolvePath("legacy DAGsDir", def.DAGsDir); err != nil {
			return err
		}
	}
	if def.Executable != "" {
		if cfg.Paths.Executable, err = l.resolvePath("legacy Executable", def.Executable); err != nil {
			return err
		}
	}
	if def.LogDir != "" {
		if cfg.Paths.LogDir, err = l.resolvePath("legacy LogDir", def.LogDir); err != nil {
			return err
		}
	}
	if def.DataDir != "" {
		if cfg.Paths.DataDir, err = l.resolvePath("legacy DataDir", def.DataDir); err != nil {
			return err
		}
	}
	if def.SuspendFlagsDir != "" {
		if cfg.Paths.SuspendFlagsDir, err = l.resolvePath("legacy SuspendFlagsDir", def.SuspendFlagsDir); err != nil {
			return err
		}
	}
	if def.AdminLogsDir != "" {
		if cfg.Paths.AdminLogsDir, err = l.resolvePath("legacy AdminLogsDir", def.AdminLogsDir); err != nil {
			return err
		}
	}
	if def.BaseConfig != "" {
		if cfg.Paths.BaseConfig, err = l.resolvePath("legacy BaseConfig", def.BaseConfig); err != nil {
			return err
		}
	}

	// UI-related legacy fields
	if l.requires(SectionUI) {
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

	return nil
}

// loadLegacyEnv maps legacy environment variables to their new counterparts in the configuration.
// If a legacy env var is set, a warning is logged and the corresponding setter function is called.
// It respects the service requirements to only apply relevant legacy environment variables.
func (l *ConfigLoader) loadLegacyEnv(cfg *Config) {
	type legacyEnvMapping struct {
		newKey   string
		setter   func(*Config, string)
		requires ConfigSection
	}

	legacyEnvs := map[string]legacyEnvMapping{
		"DAGU__ADMIN_NAVBAR_COLOR": {
			newKey:   "DAGU_NAVBAR_COLOR",
			setter:   func(c *Config, v string) { c.UI.NavbarColor = v },
			requires: SectionUI,
		},
		"DAGU__ADMIN_NAVBAR_TITLE": {
			newKey:   "DAGU_NAVBAR_TITLE",
			setter:   func(c *Config, v string) { c.UI.NavbarTitle = v },
			requires: SectionUI,
		},
		"DAGU__ADMIN_PORT": {
			newKey: "DAGU_PORT",
			setter: func(c *Config, v string) {
				if i, err := strconv.Atoi(v); err == nil {
					c.Server.Port = i
				}
			},
			requires: SectionServer,
		},
		"DAGU__ADMIN_HOST": {
			newKey:   "DAGU_HOST",
			setter:   func(c *Config, v string) { c.Server.Host = v },
			requires: SectionServer,
		},
		"DAGU__DATA": {
			newKey:   "DAGU_DATA_DIR",
			setter:   func(c *Config, v string) { c.Paths.DataDir = fileutil.ResolvePathOrBlank(v) },
			requires: SectionNone, // Always applies
		},
		"DAGU__SUSPEND_FLAGS_DIR": {
			newKey:   "DAGU_SUSPEND_FLAGS_DIR",
			setter:   func(c *Config, v string) { c.Paths.SuspendFlagsDir = fileutil.ResolvePathOrBlank(v) },
			requires: SectionNone, // Always applies
		},
		"DAGU__ADMIN_LOGS_DIR": {
			newKey:   "DAGU_ADMIN_LOG_DIR",
			setter:   func(c *Config, v string) { c.Paths.AdminLogsDir = fileutil.ResolvePathOrBlank(v) },
			requires: SectionNone, // Always applies
		},
	}

	// For each legacy variable, if it is set and requirements are met, apply it.
	for oldKey, mapping := range legacyEnvs {
		// Skip if requirements not met (SectionNone always passes)
		if mapping.requires != SectionNone && !l.requires(mapping.requires) {
			continue
		}

		if value := os.Getenv(oldKey); value != "" {
			log.Printf("%s is deprecated. Use %s instead.", oldKey, mapping.newKey)
			mapping.setter(cfg, value)
		}
	}
}

func (l *ConfigLoader) setupViper(xdgConfig XDGConfig, homeDir, configFile, appHomeOverride string) (warnings []string, err error) {
	var paths Paths
	if appHomeOverride != "" {
		resolved := fileutil.ResolvePathOrBlank(appHomeOverride)
		paths = setUnifiedPaths(resolved)
	} else {
		paths, err = ResolvePaths("DAGU_HOME", filepath.Join(homeDir, ".dagu"), xdgConfig)
		if err != nil {
			return nil, err
		}
	}

	l.configureViper(paths.ConfigDir, configFile)
	l.bindEnvironmentVariables()
	l.setViperDefaultValues(paths)

	return paths.Warnings, nil
}

// setViperDefaultValues sets the default configuration values for viper.
func (l *ConfigLoader) setViperDefaultValues(paths Paths) {
	// File paths
	l.v.SetDefault("workDir", "")         // Defaults to DAG location if empty.
	l.v.SetDefault("skipExamples", false) // Defaults to creating examples
	l.v.SetDefault("paths.dagsDir", paths.DAGsDir)
	l.v.SetDefault("paths.suspendFlagsDir", paths.SuspendFlagsDir)
	l.v.SetDefault("paths.dataDir", paths.DataDir)
	l.v.SetDefault("paths.logDir", paths.LogsDir)
	l.v.SetDefault("paths.adminLogsDir", paths.AdminLogsDir)
	l.v.SetDefault("paths.baseConfig", paths.BaseConfigFile)

	// Server settings
	l.v.SetDefault("host", "127.0.0.1")
	l.v.SetDefault("port", 8080)
	l.v.SetDefault("debug", false)
	l.v.SetDefault("basePath", "")
	l.v.SetDefault("apiBasePath", "/api/v2")
	l.v.SetDefault("latestStatusToday", false)
	l.v.SetDefault("metrics", "private")
	l.v.SetDefault("cache", "normal")

	// Coordinator settings
	l.v.SetDefault("coordinator.host", "127.0.0.1")
	l.v.SetDefault("coordinator.advertise", "") // Empty means auto-detect hostname
	l.v.SetDefault("coordinator.port", 50055)

	// Worker settings - nested structure
	l.v.SetDefault("worker.maxActiveRuns", 100)

	// UI settings
	l.v.SetDefault("ui.navbarTitle", AppName)
	l.v.SetDefault("ui.maxDashboardPageLimit", 100)
	l.v.SetDefault("ui.logEncodingCharset", getDefaultLogEncodingCharset())
	l.v.SetDefault("ui.dags.sortField", "name")
	l.v.SetDefault("ui.dags.sortOrder", "asc")

	// Logging settings
	l.v.SetDefault("logFormat", "text")

	// Queue settings
	l.v.SetDefault("queues.enabled", true)

	// Scheduler settings
	l.v.SetDefault("scheduler.lockStaleThreshold", "30s")
	l.v.SetDefault("scheduler.lockRetryInterval", "5s")

	// Peer settings
	l.v.SetDefault("peer.insecure", true) // Default to insecure (h2c)

	// Monitoring settings
	l.v.SetDefault("monitoring.retention", "24h")
	l.v.SetDefault("monitoring.interval", "5s")
}

// envBinding defines a mapping between a config key and its environment variable.
type envBinding struct {
	key    string // Viper config key
	env    string // Environment variable suffix (without DAGU_ prefix)
	isPath bool   // Whether to normalize as a file path
}

// envBindings defines all environment variable bindings for configuration.
// This declarative approach makes it easy to see and maintain all bindings.
var envBindings = []envBinding{
	// Server configurations
	{key: "logFormat", env: "LOG_FORMAT"},
	{key: "basePath", env: "BASE_PATH"},
	{key: "apiBaseURL", env: "API_BASE_URL"},
	{key: "tz", env: "TZ"},
	{key: "host", env: "HOST"},
	{key: "port", env: "PORT"},
	{key: "debug", env: "DEBUG"},
	{key: "headless", env: "HEADLESS"},
	{key: "latestStatusToday", env: "LATEST_STATUS_TODAY"},
	{key: "metrics", env: "SERVER_METRICS"},
	{key: "cache", env: "CACHE"},

	// Core configurations
	{key: "workDir", env: "WORK_DIR", isPath: true},
	{key: "defaultShell", env: "DEFAULT_SHELL"},
	{key: "skipExamples", env: "SKIP_EXAMPLES"},

	// Scheduler configurations
	{key: "scheduler.port", env: "SCHEDULER_PORT"},
	{key: "scheduler.lockStaleThreshold", env: "SCHEDULER_LOCK_STALE_THRESHOLD"},
	{key: "scheduler.lockRetryInterval", env: "SCHEDULER_LOCK_RETRY_INTERVAL"},
	{key: "scheduler.zombieDetectionInterval", env: "SCHEDULER_ZOMBIE_DETECTION_INTERVAL"},

	// UI configurations
	{key: "ui.maxDashboardPageLimit", env: "UI_MAX_DASHBOARD_PAGE_LIMIT"},
	{key: "ui.logEncodingCharset", env: "UI_LOG_ENCODING_CHARSET"},
	{key: "ui.navbarColor", env: "UI_NAVBAR_COLOR"},
	{key: "ui.navbarTitle", env: "UI_NAVBAR_TITLE"},
	{key: "ui.dags.sortField", env: "UI_DAGS_SORT_FIELD"},
	{key: "ui.dags.sortOrder", env: "UI_DAGS_SORT_ORDER"},

	// UI configurations (legacy keys)
	{key: "ui.maxDashboardPageLimit", env: "MAX_DASHBOARD_PAGE_LIMIT"},
	{key: "ui.logEncodingCharset", env: "LOG_ENCODING_CHARSET"},
	{key: "ui.navbarColor", env: "NAVBAR_COLOR"},
	{key: "ui.navbarTitle", env: "NAVBAR_TITLE"},

	// Authentication configurations
	{key: "auth.mode", env: "AUTH_MODE"},
	{key: "auth.basic.username", env: "AUTH_BASIC_USERNAME"},
	{key: "auth.basic.password", env: "AUTH_BASIC_PASSWORD"},
	{key: "auth.token.value", env: "AUTH_TOKEN"},

	// Authentication configurations (OIDC)
	{key: "auth.oidc.clientId", env: "AUTH_OIDC_CLIENT_ID"},
	{key: "auth.oidc.clientSecret", env: "AUTH_OIDC_CLIENT_SECRET"},
	{key: "auth.oidc.clientUrl", env: "AUTH_OIDC_CLIENT_URL"},
	{key: "auth.oidc.issuer", env: "AUTH_OIDC_ISSUER"},
	{key: "auth.oidc.scopes", env: "AUTH_OIDC_SCOPES"},
	{key: "auth.oidc.whitelist", env: "AUTH_OIDC_WHITELIST"},

	// Authentication configurations (legacy keys)
	{key: "auth.basic.username", env: "BASICAUTH_USERNAME"},
	{key: "auth.basic.password", env: "BASICAUTH_PASSWORD"},
	{key: "auth.token.value", env: "AUTHTOKEN"},

	// Authentication configurations (builtin)
	{key: "auth.builtin.admin.username", env: "AUTH_ADMIN_USERNAME"},
	{key: "auth.builtin.admin.password", env: "AUTH_ADMIN_PASSWORD"},
	{key: "auth.builtin.token.secret", env: "AUTH_TOKEN_SECRET"},
	{key: "auth.builtin.token.ttl", env: "AUTH_TOKEN_TTL"},

	// TLS configurations
	{key: "tls.certFile", env: "CERT_FILE"},
	{key: "tls.keyFile", env: "KEY_FILE"},

	// File paths
	{key: "paths.dagsDir", env: "DAGS", isPath: true},
	{key: "paths.dagsDir", env: "DAGS_DIR", isPath: true},
	{key: "paths.executable", env: "EXECUTABLE", isPath: true},
	{key: "paths.logDir", env: "LOG_DIR", isPath: true},
	{key: "paths.dataDir", env: "DATA_DIR", isPath: true},
	{key: "paths.suspendFlagsDir", env: "SUSPEND_FLAGS_DIR", isPath: true},
	{key: "paths.adminLogsDir", env: "ADMIN_LOG_DIR", isPath: true},
	{key: "paths.baseConfig", env: "BASE_CONFIG", isPath: true},
	{key: "paths.dagRunsDir", env: "DAG_RUNS_DIR", isPath: true},
	{key: "paths.procDir", env: "PROC_DIR", isPath: true},
	{key: "paths.queueDir", env: "QUEUE_DIR", isPath: true},
	{key: "paths.serviceRegistryDir", env: "SERVICE_REGISTRY_DIR", isPath: true},
	{key: "paths.usersDir", env: "USERS_DIR", isPath: true},

	// Queue configuration
	{key: "queues.enabled", env: "QUEUE_ENABLED"},

	// Coordinator configuration
	{key: "coordinator.host", env: "COORDINATOR_HOST"},
	{key: "coordinator.advertise", env: "COORDINATOR_ADVERTISE"},
	{key: "coordinator.port", env: "COORDINATOR_PORT"},

	// Worker configuration
	{key: "worker.id", env: "WORKER_ID"},
	{key: "worker.maxActiveRuns", env: "WORKER_MAX_ACTIVE_RUNS"},
	{key: "worker.labels", env: "WORKER_LABELS"},

	// Peer configuration
	{key: "peer.certFile", env: "PEER_CERT_FILE"},
	{key: "peer.keyFile", env: "PEER_KEY_FILE"},
	{key: "peer.clientCaFile", env: "PEER_CLIENT_CA_FILE"},
	{key: "peer.skipTlsVerify", env: "PEER_SKIP_TLS_VERIFY"},
	{key: "peer.insecure", env: "PEER_INSECURE"},

	// Monitoring configuration
	{key: "monitoring.retention", env: "MONITORING_RETENTION"},
	{key: "monitoring.interval", env: "MONITORING_INTERVAL"},
}

// bindEnvironmentVariables binds configuration keys to environment variables using the loader's viper instance.
func (l *ConfigLoader) bindEnvironmentVariables() {
	prefix := strings.ToUpper(AppSlug) + "_"

	for _, b := range envBindings {
		fullEnv := prefix + b.env

		// Normalize path if needed
		if b.isPath {
			if val := os.Getenv(fullEnv); val != "" {
				if abs, err := filepath.Abs(val); err == nil && abs != val {
					_ = os.Setenv(fullEnv, abs)
				}
			}
		}

		_ = l.v.BindEnv(b.key, fullEnv)
	}
}

// configureViper sets up the viper instance with config file paths and environment settings.
func (l *ConfigLoader) configureViper(configDir, configFile string) {
	if configFile == "" {
		l.v.AddConfigPath(configDir)
		l.v.SetConfigName("config")
	} else {
		l.v.SetConfigFile(configFile)
	}
	l.v.SetConfigType("yaml")
	l.v.SetEnvPrefix(strings.ToUpper(AppSlug))
	l.v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	l.v.AutomaticEnv()
}

// parseWorkerLabels parses a comma-separated list of `key=value` pairs into a map[string]string.
// Whitespace around keys and values is trimmed; empty entries and pairs with an empty key are ignored.
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

		key, value, found := strings.Cut(pair, "=")
		if found {
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
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
