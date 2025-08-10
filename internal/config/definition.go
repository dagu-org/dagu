package config

// Definition holds the overall configuration for the application.
// Each field maps to a configuration key defined in external sources (like YAML files)
type Definition struct {
	// Peer contains configuration for peer connections over gRPC.
	Peer peerDef `mapstructure:"peer"`

	// Host defines the hostname or IP address on which the application will run.
	Host string `mapstructure:"host"`

	// Port specifies the network port for incoming connections.
	Port int `mapstructure:"port"`

	// PermissionWriteDAGs indicates if the user has permission to write DAGs.
	PermissionWriteDAGs *bool `mapstructure:"permissionWriteDAGs"`

	// PermissionRunDAGs indicates if the user has permission to run DAGs.
	PermissionRunDAGs *bool `mapstructure:"permissionRunDAGs"`

	// Permissions defines the permissions allowed in the UI and API.
	Permissions permissionsDef `mapstructure:"permissions"`

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

	// WorkDir specifies the default working directory for DAG (Directed Acyclic Graph) files.
	// If not explicitly provided, it defaults to the directory where the DAG file resides.
	WorkDir string `mapstructure:"workDir"`

	// DefaultShell specifies the default shell to use for command execution.
	// If not provided, platform-specific defaults are used (PowerShell on Windows, $SHELL on Unix).
	DefaultShell string `mapstructure:"defaultShell"`

	// Headless determines if the application should run without a graphical user interface.
	// Useful for automated or headless server environments.
	Headless *bool `mapstructure:"headless"`

	// Auth contains authentication settings (such as credentials or tokens) needed to secure the application.
	Auth *authDef `mapstructure:"auth"`

	// Paths holds various filesystem path configurations used throughout the application.
	Paths *pathsConfigDef `mapstructure:"paths"`

	// LogFormat defines the output format for log messages (e.g., JSON, plain text).
	// Available options: "json", "text"
	LogFormat string `mapstructure:"logFormat"`

	// LatestStatusToday indicates whether the application should display only the most recent status for the current day.
	LatestStatusToday *bool `mapstructure:"latestStatusToday"`

	// TZ represents the timezone setting for the application (for example, "UTC" or "America/New_York").
	TZ string `mapstructure:"tz"`

	// UI contains settings specific to the application's user interface.
	UI *uiDef `mapstructure:"ui"`

	// RemoteNodes holds a list of configurations for connecting to remote nodes.
	// This enables the management of DAGs on external servers.
	RemoteNodes []remoteNodeDef `mapstructure:"remoteNodes"`

	// TLS contains configuration details for enabling TLS/SSL encryption,
	// such as certificate and key file paths.
	TLS *tlsConfigDef `mapstructure:"tls"`

	// Queues contains global queue configuration settings.
	Queues *queuesDef `mapstructure:"queues"`

	// Coordinator contains configuration for the coordinator server.
	Coordinator *coordinatorDef `mapstructure:"coordinator"`

	// Worker contains configuration for the worker.
	Worker *workerDef `mapstructure:"worker"`

	// Scheduler contains configuration for the scheduler.
	Scheduler *schedulerDef `mapstructure:"scheduler"`

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
}

// peerDef holds the certificate and TLS configuration for peer connections over gRPC.
type peerDef struct {
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

// authDef holds the authentication configuration for the application.
type authDef struct {
	Basic *authBasicDef `mapstructure:"basic"`
	Token *authTokenDef `mapstructure:"token"`
	OIDC  *authOIDCDef  `mapstructure:"oidc"`
}

// authBasicDef represents the basic authentication configuration
type authBasicDef struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// authTokenDef represents the authentication token configuration
type authTokenDef struct {
	Value string `mapstructure:"value"`
}

type authOIDCDef struct {
	ClientId     string   `mapstructure:"clientId"`
	ClientSecret string   `mapstructure:"clientSecret"`
	ClientUrl    string   `mapstructure:"clientUrl"`
	Issuer       string   `mapstructure:"issuer"`
	Scopes       []string `mapstructure:"scopes"`
	Whitelist    []string `mapstructure:"whitelist"`
}

// PathsConfigDef represents the file system paths configuration.
type pathsConfigDef struct {
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
}

// uiDef holds the user interface configuration settings.
type uiDef struct {
	LogEncodingCharset    string   `mapstructure:"logEncodingCharset"`
	NavbarColor           string   `mapstructure:"navbarColor"`
	NavbarTitle           string   `mapstructure:"navbarTitle"`
	MaxDashboardPageLimit int      `mapstructure:"maxDashboardPageLimit"`
	DAGs                  *dagsDef `mapstructure:"dags"`
}

// dagsDef holds the DAGs page configuration settings.
type dagsDef struct {
	SortField string `mapstructure:"sortField"`
	SortOrder string `mapstructure:"sortOrder"`
}

// permissionsDef holds the permissions configuration for the application.
// It defines what actions are allowed in the UI, such as writing DAGs.
type permissionsDef struct {
	WriteDAGs *bool `mapstructure:"writeDAGs"`
	RunDAGs   *bool `mapstructure:"runDAGs"`
}

// remoteNodeDef represents a configuration for connecting to a remote node.
type remoteNodeDef struct {
	Name              string `mapstructure:"name"`
	APIBaseURL        string `mapstructure:"apiBaseURL"`
	IsBasicAuth       bool   `mapstructure:"isBasicAuth"`
	BasicAuthUsername string `mapstructure:"basicAuthUsername"`
	BasicAuthPassword string `mapstructure:"basicAuthPassword"`
	IsAuthToken       bool   `mapstructure:"isAuthToken"`
	AuthToken         string `mapstructure:"authToken"`
	SkipTLSVerify     bool   `mapstructure:"skipTLSVerify"`
}

// tlsConfigDef represents TLS configuration
type tlsConfigDef struct {
	CertFile string `mapstructure:"certFile"`
	KeyFile  string `mapstructure:"keyFile"`
	CAFile   string `mapstructure:"caFile"`
}

// queuesDef represents the global queue configuration
type queuesDef struct {
	Enabled bool             `mapstructure:"enabled"`
	Config  []queueConfigDef `mapstructure:"config"`
}

// queueConfigDef represents individual queue configuration
type queueConfigDef struct {
	Name          string `mapstructure:"name"`
	MaxActiveRuns int    `mapstructure:"maxActiveRuns"`
}

// coordinatorDef holds the configuration for the coordinator service.
type coordinatorDef struct {
	// Host is the hostname or IP address for the coordinator service.
	Host string `mapstructure:"host"`

	// Port is the port number for the coordinator service.
	Port int `mapstructure:"port"`
}

// workerDef holds the configuration for the worker.
type workerDef struct {
	// ID is the unique identifier for the worker instance.
	ID string `mapstructure:"id"`

	// MaxActiveRuns is the maximum number of active runs for the worker.
	MaxActiveRuns int `mapstructure:"maxActiveRuns"`

	// Labels are the worker labels for capability matching.
	// Can be either a string (key1=value1,key2=value2) or a map in YAML.
	Labels interface{} `mapstructure:"labels"`
}

// schedulerDef holds the configuration for the scheduler.
type schedulerDef struct {
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
