package config

import (
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
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
	SectionGitSync                               // 256
	SectionTunnel                                // 512

	// SectionAll combines all sections (useful for ServiceNone/CLI)
	SectionAll = SectionServer | SectionScheduler | SectionWorker | SectionCoordinator | SectionUI | SectionQueues | SectionMonitoring | SectionGitSync | SectionTunnel
)

// serviceRequirements maps services to their required config sections using bitwise OR.
var serviceRequirements = map[Service]ConfigSection{
	ServiceNone:        SectionAll,
	ServiceServer:      SectionServer | SectionCoordinator | SectionUI | SectionQueues | SectionMonitoring | SectionGitSync | SectionTunnel,
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

// parseDuration parses a duration string and adds a warning if parsing fails.
// Returns the parsed duration (or zero if empty/invalid).
func (l *ConfigLoader) parseDuration(fieldName, value string) time.Duration {
	if value == "" {
		return 0
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		l.warnings = append(l.warnings, fmt.Sprintf("Invalid %s value: %s", fieldName, value))
		return 0
	}
	return duration
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

	// Set validation-safe defaults for fields that may not be loaded
	// These ensure cfg.Validate() passes even if section-specific loading is skipped
	cfg.UI.MaxDashboardPageLimit = 1    // Minimum valid value
	cfg.Server.Port = 8080              // Default port
	cfg.Server.Auth.Mode = AuthModeNone // Default auth mode

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
	if l.requires(SectionGitSync) {
		l.loadGitSyncConfig(&cfg, def)
	}
	if l.requires(SectionTunnel) {
		l.loadTunnelConfig(&cfg, def)
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

	// Validate the complete configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
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
		Peer:         l.loadPeerConfig(def.Peer),
	}

	// Initialize the timezone
	if err := setTimezone(&cfg.Core); err != nil {
		return fmt.Errorf("failed to set timezone: %w", err)
	}

	return nil
}

// loadPeerConfig converts PeerDef to Peer configuration.
func (l *ConfigLoader) loadPeerConfig(def PeerDef) Peer {
	return Peer{
		CertFile:      def.CertFile,
		KeyFile:       def.KeyFile,
		ClientCaFile:  def.ClientCaFile,
		SkipTLSVerify: def.SkipTLSVerify,
		Insecure:      def.Insecure,
		MaxRetries:    def.MaxRetries,
		RetryInterval: l.parseDuration("peer.retryInterval", def.RetryInterval),
	}
}

// loadPathsConfig loads the file system paths configuration.
// All paths are resolved to absolute paths. Returns an error if any path resolution fails.
func (l *ConfigLoader) loadPathsConfig(cfg *Config, def Definition) error {
	if def.Paths == nil {
		return nil
	}

	// Define path mappings: target pointer and source value
	pathMappings := []struct {
		name   string
		target *string
		source string
	}{
		{"DAGsDir", &cfg.Paths.DAGsDir, def.Paths.DAGsDir},
		{"SuspendFlagsDir", &cfg.Paths.SuspendFlagsDir, def.Paths.SuspendFlagsDir},
		{"DataDir", &cfg.Paths.DataDir, def.Paths.DataDir},
		{"LogDir", &cfg.Paths.LogDir, def.Paths.LogDir},
		{"AdminLogsDir", &cfg.Paths.AdminLogsDir, def.Paths.AdminLogsDir},
		{"BaseConfig", &cfg.Paths.BaseConfig, def.Paths.BaseConfig},
		{"Executable", &cfg.Paths.Executable, def.Paths.Executable},
		{"DAGRunsDir", &cfg.Paths.DAGRunsDir, def.Paths.DAGRunsDir},
		{"QueueDir", &cfg.Paths.QueueDir, def.Paths.QueueDir},
		{"ProcDir", &cfg.Paths.ProcDir, def.Paths.ProcDir},
		{"ServiceRegistryDir", &cfg.Paths.ServiceRegistryDir, def.Paths.ServiceRegistryDir},
		{"UsersDir", &cfg.Paths.UsersDir, def.Paths.UsersDir},
		{"APIKeysDir", &cfg.Paths.APIKeysDir, def.Paths.APIKeysDir},
		{"WebhooksDir", &cfg.Paths.WebhooksDir, def.Paths.WebhooksDir},
		{"ConversationsDir", &cfg.Paths.ConversationsDir, def.Paths.ConversationsDir},
	}

	for _, mapping := range pathMappings {
		resolved, err := l.resolvePath(mapping.name, mapping.source)
		if err != nil {
			return err
		}
		*mapping.target = resolved
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
			oidc := def.Auth.OIDC
			// Core OIDC fields (used by both standalone and builtin modes)
			cfg.Server.Auth.OIDC.ClientId = oidc.ClientId
			cfg.Server.Auth.OIDC.ClientSecret = oidc.ClientSecret
			cfg.Server.Auth.OIDC.ClientUrl = oidc.ClientUrl
			cfg.Server.Auth.OIDC.Issuer = oidc.Issuer
			// Use parseStringList to support both comma-separated strings and YAML lists
			cfg.Server.Auth.OIDC.Scopes = parseStringList(l.v.Get("auth.oidc.scopes"))
			cfg.Server.Auth.OIDC.Whitelist = parseStringList(l.v.Get("auth.oidc.whitelist"))
			// Builtin-specific fields (only used when auth.mode=builtin)
			if oidc.AutoSignup != nil {
				cfg.Server.Auth.OIDC.AutoSignup = *oidc.AutoSignup
			} else {
				// Default to true - if OIDC is configured, auto-signup is expected
				cfg.Server.Auth.OIDC.AutoSignup = true
			}
			cfg.Server.Auth.OIDC.AllowedDomains = parseStringList(l.v.Get("auth.oidc.allowedDomains"))
			cfg.Server.Auth.OIDC.ButtonLabel = oidc.ButtonLabel
			// Load role mapping configuration
			if oidc.RoleMapping != nil {
				rm := oidc.RoleMapping
				cfg.Server.Auth.OIDC.RoleMapping.DefaultRole = rm.DefaultRole
				cfg.Server.Auth.OIDC.RoleMapping.GroupsClaim = rm.GroupsClaim
				cfg.Server.Auth.OIDC.RoleMapping.GroupMappings = rm.GroupMappings
				cfg.Server.Auth.OIDC.RoleMapping.RoleAttributePath = rm.RoleAttributePath
				if rm.RoleAttributeStrict != nil {
					cfg.Server.Auth.OIDC.RoleMapping.RoleAttributeStrict = *rm.RoleAttributeStrict
				}
				if rm.SkipOrgRoleSync != nil {
					cfg.Server.Auth.OIDC.RoleMapping.SkipOrgRoleSync = *rm.SkipOrgRoleSync
				}
			}
		}
		if def.Auth.Builtin != nil {
			if def.Auth.Builtin.Admin != nil {
				cfg.Server.Auth.Builtin.Admin.Username = def.Auth.Builtin.Admin.Username
				cfg.Server.Auth.Builtin.Admin.Password = def.Auth.Builtin.Admin.Password
			}
			if def.Auth.Builtin.Token != nil {
				cfg.Server.Auth.Builtin.Token.Secret = def.Auth.Builtin.Token.Secret
				cfg.Server.Auth.Builtin.Token.TTL = l.parseDuration("auth.builtin.token.ttl", def.Auth.Builtin.Token.TTL)
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

	// Set defaults for OIDC configuration (used by both standalone and builtin modes)
	if len(cfg.Server.Auth.OIDC.Scopes) == 0 {
		cfg.Server.Auth.OIDC.Scopes = []string{"openid", "profile", "email"}
	}
	if cfg.Server.Auth.OIDC.RoleMapping.DefaultRole == "" {
		cfg.Server.Auth.OIDC.RoleMapping.DefaultRole = "viewer"
	}
	if cfg.Server.Auth.OIDC.ButtonLabel == "" {
		cfg.Server.Auth.OIDC.ButtonLabel = "Login with SSO"
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

	// Set terminal configuration (default: disabled)
	cfg.Server.Terminal.Enabled = false
	if def.Terminal != nil && def.Terminal.Enabled != nil {
		cfg.Server.Terminal.Enabled = *def.Terminal.Enabled
	}

	// Set audit configuration (default: enabled)
	cfg.Server.Audit.Enabled = true
	if def.Audit != nil && def.Audit.Enabled != nil {
		cfg.Server.Audit.Enabled = *def.Audit.Enabled
	}
}

// loadUIConfig loads the UI configuration.
func (l *ConfigLoader) loadUIConfig(cfg *Config, def Definition) {
	// Apply defaults from viper
	cfg.UI.MaxDashboardPageLimit = l.v.GetInt("ui.maxDashboardPageLimit")
	cfg.UI.NavbarTitle = l.v.GetString("ui.navbarTitle")
	cfg.UI.LogEncodingCharset = l.v.GetString("ui.logEncodingCharset")
	cfg.UI.DAGs.SortField = l.v.GetString("ui.dags.sortField")
	cfg.UI.DAGs.SortOrder = l.v.GetString("ui.dags.sortOrder")

	if def.UI == nil {
		return
	}

	// Apply definition overrides
	cfg.UI.NavbarColor = def.UI.NavbarColor
	setIfNotEmpty(&cfg.UI.NavbarTitle, def.UI.NavbarTitle)
	setIfNotEmpty(&cfg.UI.LogEncodingCharset, def.UI.LogEncodingCharset)

	if def.UI.MaxDashboardPageLimit > 0 {
		cfg.UI.MaxDashboardPageLimit = def.UI.MaxDashboardPageLimit
	}

	if def.UI.DAGs != nil {
		setIfNotEmpty(&cfg.UI.DAGs.SortField, def.UI.DAGs.SortField)
		setIfNotEmpty(&cfg.UI.DAGs.SortOrder, def.UI.DAGs.SortOrder)
	}
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
			cfg.Worker.Labels = parseLabels(def.Worker.Labels)
		}

		// Parse coordinators for static service discovery
		if def.Worker.Coordinators != nil {
			addresses, addrWarnings := parseCoordinatorAddresses(def.Worker.Coordinators)
			cfg.Worker.Coordinators = addresses
			l.warnings = append(l.warnings, addrWarnings...)
		}

		// Load PostgresPool config from definition
		if def.Worker.PostgresPool != nil {
			l.loadPostgresPoolConfig(&cfg.Worker.PostgresPool, def.Worker.PostgresPool)
		}
	}

	// Apply PostgresPool defaults
	l.setPostgresPoolDefaults(&cfg.Worker.PostgresPool)
}

// loadPostgresPoolConfig loads PostgreSQL pool configuration from definition.
func (l *ConfigLoader) loadPostgresPoolConfig(pool *PostgresPoolConfig, def *PostgresPoolDef) {
	if def.MaxOpenConns > 0 {
		pool.MaxOpenConns = def.MaxOpenConns
	}
	if def.MaxIdleConns > 0 {
		pool.MaxIdleConns = def.MaxIdleConns
	}
	if def.ConnMaxLifetime > 0 {
		pool.ConnMaxLifetime = def.ConnMaxLifetime
	}
	if def.ConnMaxIdleTime > 0 {
		pool.ConnMaxIdleTime = def.ConnMaxIdleTime
	}
}

// setPostgresPoolDefaults sets default values for PostgreSQL pool configuration.
func (l *ConfigLoader) setPostgresPoolDefaults(pool *PostgresPoolConfig) {
	defaults := []struct {
		target       *int
		defaultValue int
	}{
		{&pool.MaxOpenConns, 25},
		{&pool.MaxIdleConns, 5},
		{&pool.ConnMaxLifetime, 300},
		{&pool.ConnMaxIdleTime, 60},
	}

	for _, d := range defaults {
		if *d.target == 0 {
			*d.target = d.defaultValue
		}
	}
}

// parseCoordinatorAddresses parses and validates coordinator addresses from various input formats.
// It accepts either a comma-separated string or a list of strings.
// Returns the list of valid addresses and any warnings for invalid ones.
func parseCoordinatorAddresses(input interface{}) ([]string, []string) {
	var addresses []string
	var warnings []string

	processAddr := func(addr string) {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			return
		}

		// Validate host:port format using net.SplitHostPort
		host, portStr, err := net.SplitHostPort(addr)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Invalid coordinator address %q: %v", addr, err))
			return
		}

		port, err := strconv.Atoi(portStr)
		if err != nil || port <= 0 || port > 65535 {
			warnings = append(warnings, fmt.Sprintf("Invalid coordinator address %q: port must be between 1 and 65535", addr))
			return
		}

		if host == "" {
			warnings = append(warnings, fmt.Sprintf("Invalid coordinator address %q: empty host", addr))
			return
		}

		addresses = append(addresses, addr)
	}

	switch v := input.(type) {
	case string:
		// Parse comma-separated string: "host1:port1,host2:port2"
		if v != "" {
			parts := strings.Split(v, ",")
			for _, part := range parts {
				processAddr(part)
			}
		}
	case []interface{}:
		// Parse list from YAML
		for _, item := range v {
			if addr, ok := item.(string); ok {
				processAddr(addr)
			}
		}
	case []string:
		// Already a string slice
		for _, addr := range v {
			processAddr(addr)
		}
	}

	return addresses, warnings
}

// loadSchedulerConfig loads the scheduler configuration.
func (l *ConfigLoader) loadSchedulerConfig(cfg *Config, def Definition) {
	if def.Scheduler != nil {
		cfg.Scheduler.Port = def.Scheduler.Port
		cfg.Scheduler.LockStaleThreshold = l.parseDuration("scheduler.lockStaleThreshold", def.Scheduler.LockStaleThreshold)
		cfg.Scheduler.LockRetryInterval = l.parseDuration("scheduler.lockRetryInterval", def.Scheduler.LockRetryInterval)
		cfg.Scheduler.ZombieDetectionInterval = l.parseDuration("scheduler.zombieDetectionInterval", def.Scheduler.ZombieDetectionInterval)
	}

	// Apply scheduler defaults
	l.setSchedulerDefaults(cfg)
}

// setSchedulerDefaults sets default values for scheduler configuration.
func (l *ConfigLoader) setSchedulerDefaults(cfg *Config) {
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
		cfg.Monitoring.Retention = l.parseDuration("monitoring.retention", def.Monitoring.Retention)
		cfg.Monitoring.Interval = l.parseDuration("monitoring.interval", def.Monitoring.Interval)
	}

	// Apply defaults
	if cfg.Monitoring.Retention <= 0 {
		cfg.Monitoring.Retention = 24 * time.Hour
	}
	if cfg.Monitoring.Interval <= 0 {
		cfg.Monitoring.Interval = 5 * time.Second
	}
}

// loadGitSyncConfig loads the Git synchronization configuration.
func (l *ConfigLoader) loadGitSyncConfig(cfg *Config, def Definition) {
	if def.GitSync == nil {
		return
	}

	// Check if gitSync is enabled
	cfg.GitSync.Enabled = def.GitSync.Enabled != nil && *def.GitSync.Enabled
	if !cfg.GitSync.Enabled {
		return
	}

	// Set defaults for enabled gitSync
	l.setGitSyncDefaults(cfg)

	// Apply definition values (override defaults where specified)
	l.applyGitSyncDefinition(cfg, def.GitSync)
}

// setGitSyncDefaults sets default values for Git sync configuration.
func (l *ConfigLoader) setGitSyncDefaults(cfg *Config) {
	cfg.GitSync.Branch = "main"
	cfg.GitSync.PushEnabled = true
	cfg.GitSync.Auth.Type = "token"
	cfg.GitSync.AutoSync.OnStartup = true
	cfg.GitSync.AutoSync.Interval = 300
	cfg.GitSync.Commit.AuthorName = "Dagu"
	cfg.GitSync.Commit.AuthorEmail = "dagu@localhost"
}

// applyGitSyncDefinition applies Git sync configuration from definition.
func (l *ConfigLoader) applyGitSyncDefinition(cfg *Config, def *GitSyncDef) {
	// Simple string fields
	setIfNotEmpty(&cfg.GitSync.Repository, def.Repository)
	setIfNotEmpty(&cfg.GitSync.Branch, def.Branch)
	setIfNotEmpty(&cfg.GitSync.Path, def.Path)

	if def.PushEnabled != nil {
		cfg.GitSync.PushEnabled = *def.PushEnabled
	}

	// Auth configuration
	if def.Auth != nil {
		setIfNotEmpty(&cfg.GitSync.Auth.Type, def.Auth.Type)
		setIfNotEmpty(&cfg.GitSync.Auth.Token, def.Auth.Token)
		setIfNotEmpty(&cfg.GitSync.Auth.SSHKeyPath, def.Auth.SSHKeyPath)
		setIfNotEmpty(&cfg.GitSync.Auth.SSHPassphrase, def.Auth.SSHPassphrase)
	}

	// AutoSync configuration
	if def.AutoSync != nil {
		if def.AutoSync.Enabled != nil {
			cfg.GitSync.AutoSync.Enabled = *def.AutoSync.Enabled
		}
		if def.AutoSync.OnStartup != nil {
			cfg.GitSync.AutoSync.OnStartup = *def.AutoSync.OnStartup
		}
		if l.v.IsSet("gitSync.autoSync.interval") {
			cfg.GitSync.AutoSync.Interval = def.AutoSync.Interval
		}
	}

	// Commit configuration
	if def.Commit != nil {
		setIfNotEmpty(&cfg.GitSync.Commit.AuthorName, def.Commit.AuthorName)
		setIfNotEmpty(&cfg.GitSync.Commit.AuthorEmail, def.Commit.AuthorEmail)
	}
}

// setIfNotEmpty sets target to value if value is not empty.
func setIfNotEmpty(target *string, value string) {
	if value != "" {
		*target = value
	}
}

// loadTunnelConfig loads the tunnel configuration.
func (l *ConfigLoader) loadTunnelConfig(cfg *Config, def Definition) {
	// Determine if tunnel is enabled (CLI flag takes precedence)
	cfg.Tunnel.Enabled = l.resolveTunnelEnabled(def)
	if !cfg.Tunnel.Enabled {
		return
	}

	// Load Tailscale config from definition
	l.loadTailscaleConfig(cfg, def)

	// Apply CLI flag overrides
	l.applyTunnelCLIOverrides(cfg)

	// Security options
	if def.Tunnel != nil && def.Tunnel.AllowTerminal != nil {
		cfg.Tunnel.AllowTerminal = *def.Tunnel.AllowTerminal
	}
	cfg.Tunnel.AllowedIPs = parseStringList(l.v.Get("tunnel.allowedIPs"))

	// Rate limiting
	l.loadTunnelRateLimiting(cfg, def)

	// Set default hostname
	if cfg.Tunnel.Tailscale.Hostname == "" {
		cfg.Tunnel.Tailscale.Hostname = AppSlug
	}
}

// resolveTunnelEnabled determines if tunnel is enabled from CLI or config.
func (l *ConfigLoader) resolveTunnelEnabled(def Definition) bool {
	if l.v.IsSet("tunnel.enabled") {
		return l.v.GetBool("tunnel.enabled")
	}
	if def.Tunnel != nil && def.Tunnel.Enabled != nil {
		return *def.Tunnel.Enabled
	}
	return false
}

// loadTailscaleConfig loads Tailscale configuration from definition.
func (l *ConfigLoader) loadTailscaleConfig(cfg *Config, def Definition) {
	if def.Tunnel == nil || def.Tunnel.Tailscale == nil {
		return
	}

	ts := def.Tunnel.Tailscale
	cfg.Tunnel.Tailscale.AuthKey = ts.AuthKey
	cfg.Tunnel.Tailscale.Hostname = ts.Hostname
	cfg.Tunnel.Tailscale.StateDir = ts.StateDir

	if ts.Funnel != nil {
		cfg.Tunnel.Tailscale.Funnel = *ts.Funnel
	}
	if ts.HTTPS != nil {
		cfg.Tunnel.Tailscale.HTTPS = *ts.HTTPS
	}
}

// applyTunnelCLIOverrides applies CLI flag overrides for tunnel configuration.
func (l *ConfigLoader) applyTunnelCLIOverrides(cfg *Config) {
	if l.v.IsSet("tunnel.tailscale.funnel") {
		cfg.Tunnel.Tailscale.Funnel = l.v.GetBool("tunnel.tailscale.funnel")
	}
	if l.v.IsSet("tunnel.tailscale.https") {
		cfg.Tunnel.Tailscale.HTTPS = l.v.GetBool("tunnel.tailscale.https")
	}
	if l.v.IsSet("tunnel.token") {
		cfg.Tunnel.Tailscale.AuthKey = l.v.GetString("tunnel.token")
	}
}

// loadTunnelRateLimiting loads rate limiting configuration for tunnel.
func (l *ConfigLoader) loadTunnelRateLimiting(cfg *Config, def Definition) {
	// Load from definition
	if def.Tunnel != nil && def.Tunnel.RateLimiting != nil {
		rl := def.Tunnel.RateLimiting
		if rl.Enabled != nil {
			cfg.Tunnel.RateLimiting.Enabled = *rl.Enabled
		}
		cfg.Tunnel.RateLimiting.LoginAttempts = rl.LoginAttempts
		cfg.Tunnel.RateLimiting.WindowSeconds = rl.WindowSeconds
		cfg.Tunnel.RateLimiting.BlockDurationSeconds = rl.BlockDurationSeconds
	}

	// Apply defaults for unset values
	rateLimitDefaults := []struct {
		target       *int
		defaultValue int
	}{
		{&cfg.Tunnel.RateLimiting.LoginAttempts, 5},
		{&cfg.Tunnel.RateLimiting.WindowSeconds, 300},
		{&cfg.Tunnel.RateLimiting.BlockDurationSeconds, 900},
	}

	for _, d := range rateLimitDefaults {
		if *d.target <= 0 {
			*d.target = d.defaultValue
		}
	}
}

// loadCacheConfig loads the cache configuration.
func (l *ConfigLoader) loadCacheConfig(cfg *Config, def Definition) {
	cfg.Cache = CacheModeNormal
	if def.Cache == nil {
		return
	}

	mode := CacheMode(*def.Cache)
	if mode.IsValid() {
		cfg.Cache = mode
	} else {
		l.warnings = append(l.warnings, fmt.Sprintf("Invalid cache mode: %q, using 'normal'", *def.Cache))
	}
}

// finalizePaths sets up derived paths and ensures required paths are set.
func (l *ConfigLoader) finalizePaths(cfg *Config) {
	// Define default path mappings relative to DataDir
	derivedPaths := []struct {
		target      *string
		defaultPath string
	}{
		{&cfg.Paths.DAGRunsDir, "dag-runs"},
		{&cfg.Paths.ProcDir, "proc"},
		{&cfg.Paths.QueueDir, "queue"},
		{&cfg.Paths.ServiceRegistryDir, "service-registry"},
		{&cfg.Paths.UsersDir, "users"},
		{&cfg.Paths.APIKeysDir, "apikeys"},
		{&cfg.Paths.WebhooksDir, "webhooks"},
	}

	for _, dp := range derivedPaths {
		if *dp.target == "" {
			*dp.target = filepath.Join(cfg.Paths.DataDir, dp.defaultPath)
		}
	}

	// ConversationsDir has a nested default path
	if cfg.Paths.ConversationsDir == "" {
		cfg.Paths.ConversationsDir = filepath.Join(cfg.Paths.DataDir, "agent", "conversations")
	}

	// Set executable path if not already configured
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

	// Worker PostgreSQL pool settings (for shared-nothing mode)
	l.v.SetDefault("worker.postgresPool.maxOpenConns", 25)
	l.v.SetDefault("worker.postgresPool.maxIdleConns", 5)
	l.v.SetDefault("worker.postgresPool.connMaxLifetime", 300)
	l.v.SetDefault("worker.postgresPool.connMaxIdleTime", 60)
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

	// Terminal configuration
	{key: "terminal.enabled", env: "TERMINAL_ENABLED"},

	// Audit configuration
	{key: "audit.enabled", env: "AUDIT_ENABLED"},

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
	// Core OIDC fields (used by both standalone and builtin modes)
	{key: "auth.oidc.clientId", env: "AUTH_OIDC_CLIENT_ID"},
	{key: "auth.oidc.clientSecret", env: "AUTH_OIDC_CLIENT_SECRET"},
	{key: "auth.oidc.clientUrl", env: "AUTH_OIDC_CLIENT_URL"},
	{key: "auth.oidc.issuer", env: "AUTH_OIDC_ISSUER"},
	{key: "auth.oidc.scopes", env: "AUTH_OIDC_SCOPES"},
	{key: "auth.oidc.whitelist", env: "AUTH_OIDC_WHITELIST"},
	// Builtin-specific OIDC fields (only used when auth.mode=builtin)
	{key: "auth.oidc.autoSignup", env: "AUTH_OIDC_AUTO_SIGNUP"},
	{key: "auth.oidc.allowedDomains", env: "AUTH_OIDC_ALLOWED_DOMAINS"},
	{key: "auth.oidc.buttonLabel", env: "AUTH_OIDC_BUTTON_LABEL"},
	// OIDC Role Mapping configuration (builtin-specific)
	{key: "auth.oidc.roleMapping.defaultRole", env: "AUTH_OIDC_DEFAULT_ROLE"},
	{key: "auth.oidc.roleMapping.groupsClaim", env: "AUTH_OIDC_GROUPS_CLAIM"},
	{key: "auth.oidc.roleMapping.groupMappings", env: "AUTH_OIDC_GROUP_MAPPINGS"},
	{key: "auth.oidc.roleMapping.roleAttributePath", env: "AUTH_OIDC_ROLE_ATTRIBUTE_PATH"},
	{key: "auth.oidc.roleMapping.roleAttributeStrict", env: "AUTH_OIDC_ROLE_ATTRIBUTE_STRICT"},
	{key: "auth.oidc.roleMapping.skipOrgRoleSync", env: "AUTH_OIDC_SKIP_ORG_ROLE_SYNC"},

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
	{key: "worker.coordinators", env: "WORKER_COORDINATORS"},

	// Peer configuration
	{key: "peer.certFile", env: "PEER_CERT_FILE"},
	{key: "peer.keyFile", env: "PEER_KEY_FILE"},
	{key: "peer.clientCaFile", env: "PEER_CLIENT_CA_FILE"},
	{key: "peer.skipTlsVerify", env: "PEER_SKIP_TLS_VERIFY"},
	{key: "peer.insecure", env: "PEER_INSECURE"},

	// Monitoring configuration
	{key: "monitoring.retention", env: "MONITORING_RETENTION"},
	{key: "monitoring.interval", env: "MONITORING_INTERVAL"},

	// Worker PostgreSQL pool configuration (for shared-nothing mode)
	{key: "worker.postgresPool.maxOpenConns", env: "WORKER_POSTGRES_POOL_MAX_OPEN_CONNS"},
	{key: "worker.postgresPool.maxIdleConns", env: "WORKER_POSTGRES_POOL_MAX_IDLE_CONNS"},
	{key: "worker.postgresPool.connMaxLifetime", env: "WORKER_POSTGRES_POOL_CONN_MAX_LIFETIME"},
	{key: "worker.postgresPool.connMaxIdleTime", env: "WORKER_POSTGRES_POOL_CONN_MAX_IDLE_TIME"},

	// Tunnel configuration (Tailscale only)
	{key: "tunnel.enabled", env: "TUNNEL"},         // Maps to --tunnel flag
	{key: "tunnel.enabled", env: "TUNNEL_ENABLED"}, // Also support TUNNEL_ENABLED
	{key: "tunnel.tailscale.authKey", env: "TUNNEL_TAILSCALE_AUTH_KEY"},
	{key: "tunnel.tailscale.hostname", env: "TUNNEL_TAILSCALE_HOSTNAME"},
	{key: "tunnel.tailscale.funnel", env: "TUNNEL_TAILSCALE_FUNNEL"},
	{key: "tunnel.tailscale.https", env: "TUNNEL_TAILSCALE_HTTPS"},
	{key: "tunnel.tailscale.stateDir", env: "TUNNEL_TAILSCALE_STATE_DIR", isPath: true},
	{key: "tunnel.allowTerminal", env: "TUNNEL_ALLOW_TERMINAL"},
	{key: "tunnel.allowedIPs", env: "TUNNEL_ALLOWED_IPS"},
	{key: "tunnel.rateLimiting.enabled", env: "TUNNEL_RATE_LIMITING_ENABLED"},
	{key: "tunnel.rateLimiting.loginAttempts", env: "TUNNEL_RATE_LIMITING_LOGIN_ATTEMPTS"},
	{key: "tunnel.rateLimiting.windowSeconds", env: "TUNNEL_RATE_LIMITING_WINDOW_SECONDS"},
	{key: "tunnel.rateLimiting.blockDurationSeconds", env: "TUNNEL_RATE_LIMITING_BLOCK_DURATION_SECONDS"},

	// Git sync configuration
	{key: "gitSync.enabled", env: "GITSYNC_ENABLED"},
	{key: "gitSync.repository", env: "GITSYNC_REPOSITORY"},
	{key: "gitSync.branch", env: "GITSYNC_BRANCH"},
	{key: "gitSync.path", env: "GITSYNC_PATH"},
	{key: "gitSync.pushEnabled", env: "GITSYNC_PUSH_ENABLED"},
	{key: "gitSync.auth.type", env: "GITSYNC_AUTH_TYPE"},
	{key: "gitSync.auth.token", env: "GITSYNC_AUTH_TOKEN"},
	{key: "gitSync.auth.sshKeyPath", env: "GITSYNC_AUTH_SSH_KEY_PATH", isPath: true},
	{key: "gitSync.auth.sshPassphrase", env: "GITSYNC_AUTH_SSH_PASSPHRASE"},
	{key: "gitSync.autoSync.enabled", env: "GITSYNC_AUTOSYNC_ENABLED"},
	{key: "gitSync.autoSync.onStartup", env: "GITSYNC_AUTOSYNC_ON_STARTUP"},
	{key: "gitSync.autoSync.interval", env: "GITSYNC_AUTOSYNC_INTERVAL"},
	{key: "gitSync.commit.authorName", env: "GITSYNC_COMMIT_AUTHOR_NAME"},
	{key: "gitSync.commit.authorEmail", env: "GITSYNC_COMMIT_AUTHOR_EMAIL"},
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

// parseLabels parses labels from various input formats into a map[string]string.
// Supports: comma-separated string ("key=value,key2=value2"), map[string]interface{}, map[interface{}]interface{}.
func parseLabels(input interface{}) map[string]string {
	labels := make(map[string]string)

	switch v := input.(type) {
	case string:
		if v == "" {
			return labels
		}
		for _, pair := range strings.Split(v, ",") {
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
	case map[string]interface{}:
		for key, val := range v {
			if strVal, ok := val.(string); ok {
				labels[key] = strVal
			}
		}
	case map[interface{}]interface{}:
		for key, val := range v {
			if keyStr, ok := key.(string); ok {
				if valStr, ok := val.(string); ok {
					labels[keyStr] = valStr
				}
			}
		}
	}

	return labels
}

// parseStringList parses input that can be either a comma-separated string or a list of strings.
// This allows config values to be specified as either:
//   - YAML list: ["a", "b", "c"]
//   - Comma-separated string: "a,b,c"
//
// Empty strings and whitespace-only entries are filtered out.
func parseStringList(input interface{}) []string {
	var result []string
	switch v := input.(type) {
	case string:
		if v != "" {
			for _, s := range strings.Split(v, ",") {
				if trimmed := strings.TrimSpace(s); trimmed != "" {
					result = append(result, trimmed)
				}
			}
		}
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				if trimmed := strings.TrimSpace(s); trimmed != "" {
					result = append(result, trimmed)
				}
			}
		}
	case []string:
		for _, s := range v {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				result = append(result, trimmed)
			}
		}
	}
	return result
}

func cleanServerBasePath(s string) string {
	if s == "" {
		return ""
	}

	// Clean and ensure absolute path
	cleanPath := path.Clean(s)
	if !path.IsAbs(cleanPath) {
		cleanPath = path.Join("/", cleanPath)
	}

	// Root path is equivalent to no base path
	if cleanPath == "/" {
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
