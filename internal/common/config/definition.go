package config

// Definition holds the overall configuration for the application.
// Each field maps to a configuration key defined in external sources (like YAML files)
type Definition struct {
	// Peer contains configuration for peer connections over gRPC.
	Peer PeerDef `mapstructure:"peer"`

	// Host defines the hostname or IP address on which the application will run.
	Host string `mapstructure:"host"`

	// Port specifies the network port for incoming connections.
	Port int `mapstructure:"port"`

	// PermissionWriteDAGs indicates if the user has permission to write DAGs.
	PermissionWriteDAGs *bool `mapstructure:"permissionWriteDAGs"`

	// PermissionRunDAGs indicates if the user has permission to run DAGs.
	PermissionRunDAGs *bool `mapstructure:"permissionRunDAGs"`

	// Permissions defines the permissions allowed in the UI and API.
	Permissions PermissionsDef `mapstructure:"permissions"`

	// Debug toggles debug mode; when true, the application may output extra logs and error details.
	Debug bool `mapstructure:"debug"`

	// BasePath is the root URL path from which the application is served.
	// This is useful when hosting the app behind a reverse proxy under a subpath.
	BasePath string `mapstructure:"basePath"`

	// APIBasePath sets the base path for all API endpoints provided by the application.
	APIBasePath string `mapstructure:"apiBasePath"`

	// APIBaseURL is a deprecated field that previously specified the full base URL for the API.
	// Use APIBasePath instead.
	APIBaseURL string `mapstructure:"apiBaseURL"`

	// DefaultShell specifies the default shell to use for command execution.
	// If not provided, platform-specific defaults are used (PowerShell on Windows, $SHELL on Unix).
	DefaultShell string `mapstructure:"defaultShell"`

	// Headless determines if the application should run without a graphical user interface.
	// Useful for automated or headless server environments.
	Headless *bool `mapstructure:"headless"`

	// Auth contains authentication settings (such as credentials or tokens) needed to secure the application.
	Auth *AuthDef `mapstructure:"auth"`

	// Paths holds various filesystem path configurations used throughout the application.
	Paths *PathsDef `mapstructure:"paths"`

	// LogFormat defines the output format for log messages (e.g., JSON, plain text).
	// Available options: "json", "text"
	LogFormat string `mapstructure:"logFormat"`

	// LatestStatusToday indicates whether the application should display only the most recent status for the current day.
	LatestStatusToday *bool `mapstructure:"latestStatusToday"`

	// TZ represents the timezone setting for the application (for example, "UTC" or "America/New_York").
	TZ string `mapstructure:"tz"`

	// UI contains settings specific to the application's user interface.
	UI *UIDef `mapstructure:"ui"`

	// RemoteNodes holds a list of configurations for connecting to remote nodes.
	// This enables the management of DAGs on external servers.
	RemoteNodes []RemoteNodeDef `mapstructure:"remoteNodes"`

	// TLS contains configuration details for enabling TLS/SSL encryption,
	// such as certificate and key file paths.
	TLS *TLSDef `mapstructure:"tls"`

	// Queues contains global queue configuration settings.
	Queues *QueueConfigDef `mapstructure:"queues"`

	// Coordinator contains configuration for the coordinator server.
	Coordinator *CoordinatorDef `mapstructure:"coordinator"`

	// Worker contains configuration for the worker.
	Worker *WorkerDef `mapstructure:"worker"`

	// Scheduler contains configuration for the scheduler.
	Scheduler *SchedulerDef `mapstructure:"scheduler"`

	// DAGs is a field that was previously used to configure the directory for DAG files.
	DAGs string `mapstructure:"dags"`

	// DAGsDir specifies the directory where DAG files are stored.
	DAGsDir string `mapstructure:"dagsDir"`

	// Executable indicates the path to the executable used for running DAG tasks.
	Executable string `mapstructure:"executable"`

	// LogDir defines the directory where log files are saved.
	LogDir string `mapstructure:"logDir"`

	// DataDir specifies the directory for storing application data, such as history or state.
	DataDir string `mapstructure:"dataDir"`

	// SuspendFlagsDir sets the directory used for storing flags that indicate a DAG is suspended.
	SuspendFlagsDir string `mapstructure:"suspendFlagsDir"`

	// AdminLogsDir indicates the directory for storing administrative logs.
	AdminLogsDir string `mapstructure:"adminLogsDir"`

	// BaseConfig provides the path to a base configuration file shared across DAGs.
	BaseConfig string `mapstructure:"baseConfig"`

	// IsBasicAuth indicates whether basic authentication is enabled.
	IsBasicAuth bool `mapstructure:"isBasicAuth"`

	// BasicAuthUsername holds the username for basic authentication.
	BasicAuthUsername string `mapstructure:"basicAuthUsername"`

	// BasicAuthPassword holds the password for basic authentication.
	BasicAuthPassword string `mapstructure:"basicAuthPassword"`

	// IsAuthToken indicates whether token-based authentication is enabled.
	IsAuthToken bool `mapstructure:"isAuthToken"`

	// AuthToken holds the token value for API authentication.
	AuthToken string `mapstructure:"authToken"`

	// LogEncodingCharset defines the character encoding used in log files.
	LogEncodingCharset string `mapstructure:"logEncodingCharset"`

	// NavbarColor sets the color of the navigation bar in the application's UI.
	NavbarColor string `mapstructure:"navbarColor"`

	// NavbarTitle specifies the title text displayed in the navigation bar of the UI.
	NavbarTitle string `mapstructure:"navbarTitle"`

	// MaxDashboardPageLimit limits the number of dashboard pages that can be shown in the UI.
	MaxDashboardPageLimit int `mapstructure:"maxDashboardPageLimit"`

	// Monitoring contains configuration for system monitoring.
	Monitoring *MonitoringDef `mapstructure:"monitoring"`

	// Metrics controls access to the /api/v2/metrics endpoint.
	// Valid values: "public", "private" (default: "private")
	Metrics *string `mapstructure:"metrics"`

	// Cache specifies the cache mode preset.
	// Valid values: "low", "normal", "high" (default: "normal")
	Cache *string `mapstructure:"cache"`

	// Terminal contains configuration for the web-based terminal feature.
	Terminal *TerminalDef `mapstructure:"terminal"`
}

// TerminalDef represents the terminal configuration.
type TerminalDef struct {
	// Enabled determines if the terminal feature is available.
	// Default: true (when using builtin auth mode)
	Enabled *bool `mapstructure:"enabled"`
}

// PeerDef holds the certificate and TLS configuration for peer connections over gRPC.
type PeerDef struct {
	// CertFile is the path to the server's TLS certificate file.
	CertFile string `mapstructure:"certFile"`

	// KeyFile is the path to the server's TLS key file.
	KeyFile string `mapstructure:"keyFile"`

	// ClientCaFile is the path to the CA certificate file used for client verification.
	ClientCaFile string `mapstructure:"clientCaFile"`

	// SkipTLSVerify indicates whether to skip TLS certificate verification.
	SkipTLSVerify bool `mapstructure:"skipTlsVerify"`

	// Insecure indicates whether to use insecure connection (h2c) instead of TLS.
	Insecure bool `mapstructure:"insecure"`
}

// AuthDef holds the authentication configuration for the application.
type AuthDef struct {
	// Mode specifies the authentication mode: "none", "builtin", or "oidc"
	Mode    *string         `mapstructure:"mode"`
	Basic   *AuthBasicDef   `mapstructure:"basic"`
	Token   *AuthTokenDef   `mapstructure:"token"`
	OIDC    *AuthOIDCDef    `mapstructure:"oidc"`
	Builtin *AuthBuiltinDef `mapstructure:"builtin"`
}

// AuthBuiltinDef represents the builtin authentication configuration
type AuthBuiltinDef struct {
	Admin *AdminConfigDef `mapstructure:"admin"`
	Token *TokenConfigDef `mapstructure:"token"`
}

// OIDCRoleMappingDef defines how OIDC claims are mapped to Dagu roles
type OIDCRoleMappingDef struct {
	// DefaultRole is the role assigned to new OIDC users when no mapping matches (default: "viewer")
	DefaultRole string `mapstructure:"defaultRole"`
	// GroupsClaim specifies the claim name containing groups (default: "groups")
	// Common values: "groups", "roles", "cognito:groups", "realm_access.roles"
	GroupsClaim string `mapstructure:"groupsClaim"`
	// GroupMappings maps IdP group names to Dagu roles
	// Example: {"admins": "admin", "developers": "manager", "ops": "operator"}
	GroupMappings map[string]string `mapstructure:"groupMappings"`
	// RoleAttributePath is a jq expression to extract role from claims (advanced)
	// Example: 'if (.groups | contains(["admins"])) then "admin" else "viewer" end'
	RoleAttributePath string `mapstructure:"roleAttributePath"`
	// RoleAttributeStrict denies login when no valid role is found (default: false)
	RoleAttributeStrict *bool `mapstructure:"roleAttributeStrict"`
	// SkipOrgRoleSync skips role sync on subsequent logins (default: false)
	// When true, roles are only assigned on first login
	SkipOrgRoleSync *bool `mapstructure:"skipOrgRoleSync"`
}

// AdminConfigDef represents the initial admin user configuration
type AdminConfigDef struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// TokenConfigDef represents the JWT token configuration
type TokenConfigDef struct {
	Secret string `mapstructure:"secret"`
	TTL    string `mapstructure:"ttl"`
}

// AuthBasicDef represents the basic authentication configuration
type AuthBasicDef struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// AuthTokenDef represents the authentication token configuration
type AuthTokenDef struct {
	Value string `mapstructure:"value"`
}

// AuthOIDCDef represents the OIDC authentication configuration.
// Core fields are used by both standalone OIDC mode and builtin auth mode with OIDC.
// Builtin-specific fields are only used when auth.mode=builtin.
// OIDC is automatically enabled under builtin mode when all required fields
// (clientId, clientSecret, clientUrl, issuer) are configured.
type AuthOIDCDef struct {
	// Core OIDC fields (used by both standalone and builtin modes)
	ClientId     string   `mapstructure:"clientId"`
	ClientSecret string   `mapstructure:"clientSecret"`
	ClientUrl    string   `mapstructure:"clientUrl"`
	Issuer       string   `mapstructure:"issuer"`
	Scopes       []string `mapstructure:"scopes"`
	Whitelist    []string `mapstructure:"whitelist"`

	// Builtin-specific fields (only used when auth.mode=builtin)
	AutoSignup     *bool               `mapstructure:"autoSignup"`     // Auto-create users on first login (default: true)
	AllowedDomains []string            `mapstructure:"allowedDomains"` // Email domain whitelist
	ButtonLabel    string              `mapstructure:"buttonLabel"`    // Login button text
	RoleMapping    *OIDCRoleMappingDef `mapstructure:"roleMapping"`    // Role mapping configuration
}

// PathsDef represents the file system paths configuration.
type PathsDef struct {
	DAGsDir            string `mapstructure:"dagsDir"`
	Executable         string `mapstructure:"executable"`
	LogDir             string `mapstructure:"logDir"`
	DataDir            string `mapstructure:"dataDir"`
	SuspendFlagsDir    string `mapstructure:"suspendFlagsDir"`
	AdminLogsDir       string `mapstructure:"adminLogsDir"`
	BaseConfig         string `mapstructure:"baseConfig"`
	DAGRunsDir         string `mapstructure:"dagRunsDir"`
	QueueDir           string `mapstructure:"queueDir"`
	ProcDir            string `mapstructure:"procDir"`
	ServiceRegistryDir string `mapstructure:"serviceRegistryDir"`
	UsersDir           string `mapstructure:"usersDir"`
	APIKeysDir         string `mapstructure:"apiKeysDir"`
	WebhooksDir        string `mapstructure:"webhooksDir"`
}

// UIDef holds the user interface configuration settings.
type UIDef struct {
	LogEncodingCharset    string      `mapstructure:"logEncodingCharset"`
	NavbarColor           string      `mapstructure:"navbarColor"`
	NavbarTitle           string      `mapstructure:"navbarTitle"`
	MaxDashboardPageLimit int         `mapstructure:"maxDashboardPageLimit"`
	DAGs                  *DAGListDef `mapstructure:"dags"`
}

// DAGListDef holds the DAGs page configuration settings.
type DAGListDef struct {
	SortField string `mapstructure:"sortField"`
	SortOrder string `mapstructure:"sortOrder"`
}

// PermissionsDef holds the permissions configuration for the application.
// It defines what actions are allowed in the UI, such as writing DAGs.
type PermissionsDef struct {
	WriteDAGs *bool `mapstructure:"writeDAGs"`
	RunDAGs   *bool `mapstructure:"runDAGs"`
}

// RemoteNodeDef represents a configuration for connecting to a remote node.
type RemoteNodeDef struct {
	Name              string `mapstructure:"name"`
	APIBaseURL        string `mapstructure:"apiBaseURL"`
	IsBasicAuth       bool   `mapstructure:"isBasicAuth"`
	BasicAuthUsername string `mapstructure:"basicAuthUsername"`
	BasicAuthPassword string `mapstructure:"basicAuthPassword"`
	IsAuthToken       bool   `mapstructure:"isAuthToken"`
	AuthToken         string `mapstructure:"authToken"`
	SkipTLSVerify     bool   `mapstructure:"skipTLSVerify"`
}

// TLSDef represents TLS configuration
type TLSDef struct {
	CertFile string `mapstructure:"certFile"`
	KeyFile  string `mapstructure:"keyFile"`
	CAFile   string `mapstructure:"caFile"`
}

// QueueConfigDef represents the global queue configuration
type QueueConfigDef struct {
	Enabled bool       `mapstructure:"enabled"`
	Config  []QueueDef `mapstructure:"config"`
}

// QueueDef represents individual queue configuration
type QueueDef struct {
	Name string `mapstructure:"name"`
	// Deprecated: use maxConcurrency
	MaxActiveRuns  *int `mapstructure:"maxActiveRuns"`
	MaxConcurrency int  `mapstructure:"maxConcurrency"`
}

// CoordinatorDef holds the configuration for the coordinator service.
type CoordinatorDef struct {
	// Host is the bind address for the coordinator gRPC server.
	Host string `mapstructure:"host"`

	// Advertise is the address to advertise in the service registry.
	// If empty, the hostname will be auto-detected.
	Advertise string `mapstructure:"advertise"`

	// Port is the port number for the coordinator service.
	Port int `mapstructure:"port"`
}

// MonitoringDef holds the configuration for system monitoring.
type MonitoringDef struct {
	// Retention specifies how long to keep system resource history.
	// Default is 24h.
	Retention string `mapstructure:"retention"`
	// Interval specifies how often to collect resource metrics.
	// Default is 5s.
	Interval string `mapstructure:"interval"`
}

// WorkerDef holds the configuration for the worker.
type WorkerDef struct {
	// ID is the unique identifier for the worker instance.
	ID string `mapstructure:"id"`

	// MaxActiveRuns is the maximum number of active runs for the worker.
	MaxActiveRuns int `mapstructure:"maxActiveRuns"`

	// Labels are the worker labels for capability matching.
	// Can be either a string (key1=value1,key2=value2) or a map in YAML.
	Labels interface{} `mapstructure:"labels"`

	// Coordinators is a list of coordinator addresses for static service discovery.
	// Can be either a string (host1:port1,host2:port2) or a list in YAML.
	// When specified, the worker will connect directly to these coordinators
	// instead of using the file-based service registry.
	Coordinators interface{} `mapstructure:"coordinators"`
}

// SchedulerDef holds the configuration for the scheduler.
type SchedulerDef struct {
	// Port is the port number for the health check server.
	Port int `mapstructure:"port"`

	// LockStaleThreshold is the time after which a scheduler lock is considered stale.
	// Default is 30 seconds.
	LockStaleThreshold string `mapstructure:"lockStaleThreshold"`

	// LockRetryInterval is the interval between lock acquisition attempts.
	// Default is 5 second.
	LockRetryInterval string `mapstructure:"lockRetryInterval"`

	// ZombieDetectionInterval is the interval between checks for zombie DAG runs.
	// Default is 45 seconds. Set to 0 to disable.
	ZombieDetectionInterval string `mapstructure:"zombieDetectionInterval"`
}
