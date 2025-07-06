package config

// Definition holds the overall configuration for the application.
// Each field maps to a configuration key defined in external sources (like YAML files)
// via the "mapstructure" tags. Some fields are legacy and maintained only for backward compatibility.
type Definition struct {
	// Host defines the hostname or IP address on which the application will run.
	Host string `mapstructure:"host"`

	// Port specifies the network port for incoming connections.
	Port int `mapstructure:"port"`

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

	// Coordinator contains the coordinator service configuration.
	Coordinator *coordinatorDef `mapstructure:"coordinator"`

	// ----------------------------------------------------------------------------
	// Legacy fields for backward compatibility - Start
	// These fields are maintained for compatibility with older configuration formats.
	// They are deprecated and should be avoided in new implementations.
	// ----------------------------------------------------------------------------

	// DAGs is a legacy field that was previously used to configure DAG-related settings.
	// Deprecated: Use Auth.Basic.Enabled instead.
	DAGs string `mapstructure:"dags"`

	// DAGsDir specifies the directory where DAG files are stored.
	// Deprecated: Use Paths.DAGsDir instead.
	DAGsDir string `mapstructure:"dagsDir"`

	// Executable indicates the path to the executable used for running DAG tasks.
	// Deprecated: Use Paths.Executable instead.
	Executable string `mapstructure:"executable"`

	// LogDir defines the directory where log files are saved.
	// Deprecated: Use Paths.LogDir instead.
	LogDir string `mapstructure:"logDir"`

	// DataDir specifies the directory for storing application data, such as history or state.
	// Deprecated: Use Paths.DataDir instead.
	DataDir string `mapstructure:"dataDir"`

	// SuspendFlagsDir sets the directory used for storing flags that indicate a DAG is suspended.
	// Deprecated: Use Paths.SuspendFlagsDir instead.
	SuspendFlagsDir string `mapstructure:"suspendFlagsDir"`

	// AdminLogsDir indicates the directory for storing administrative logs.
	// Deprecated: Use Paths.AdminLogsDir instead.
	AdminLogsDir string `mapstructure:"adminLogsDir"`

	// BaseConfig provides the path to a base configuration file shared across DAGs.
	// Deprecated: Use Paths.BaseConfig instead.
	BaseConfig string `mapstructure:"baseConfig"`

	// IsBasicAuth indicates whether basic authentication is enabled.
	// Deprecated: Use Auth.Token.Enabled instead.
	IsBasicAuth bool `mapstructure:"isBasicAuth"`

	// BasicAuthUsername holds the username for basic authentication.
	// Deprecated: Use Auth.Basic.Username instead.
	BasicAuthUsername string `mapstructure:"basicAuthUsername"`

	// BasicAuthPassword holds the password for basic authentication.
	// Deprecated: Use Auth.Basic.Password instead.
	BasicAuthPassword string `mapstructure:"basicAuthPassword"`

	// IsAuthToken indicates whether token-based authentication is enabled.
	// Deprecated: Use Auth.Token.Enabled instead.
	IsAuthToken bool `mapstructure:"isAuthToken"`

	// AuthToken holds the token value for API authentication.
	// Deprecated: Use Auth.Token.Value instead.
	AuthToken string `mapstructure:"authToken"`

	// LogEncodingCharset defines the character encoding used in log files.
	// Deprecated: Use UI.LogEncodingCharset instead.
	LogEncodingCharset string `mapstructure:"logEncodingCharset"`

	// NavbarColor sets the color of the navigation bar in the application's UI.
	// Deprecated: Use UI.NavbarColor instead.
	NavbarColor string `mapstructure:"navbarColor"`

	// NavbarTitle specifies the title text displayed in the navigation bar of the UI.
	// Deprecated: Use UI.NavbarTitle instead.
	NavbarTitle string `mapstructure:"navbarTitle"`

	// MaxDashboardPageLimit limits the number of dashboard pages that can be shown in the UI.
	// Deprecated: Use UI.MaxDashboardPageLimit instead.
	MaxDashboardPageLimit int `mapstructure:"maxDashboardPageLimit"`

	// ----------------------------------------------------------------------------
	// Legacy fields for backward compatibility - End
	// ----------------------------------------------------------------------------
}

// authDef holds the authentication configuration for the application.
type authDef struct {
	Basic *authBasicDef `mapstructure:"basic"`
	Token *authTokenDef `mapstructure:"token"`
}

// authBasicDef represents the basic authentication configuration
type authBasicDef struct {
	Enabled  bool   `mapstructure:"enabled"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// authTokenDef represents the authentication token configuration
type authTokenDef struct {
	Enabled bool   `mapstructure:"enabled"`
	Value   string `mapstructure:"value"`
}

// PathsConfigDef represents the file system paths configuration.
type pathsConfigDef struct {
	DAGsDir         string `mapstructure:"dagsDir"`
	Executable      string `mapstructure:"executable"`
	LogDir          string `mapstructure:"logDir"`
	DataDir         string `mapstructure:"dataDir"`
	SuspendFlagsDir string `mapstructure:"suspendFlagsDir"`
	AdminLogsDir    string `mapstructure:"adminLogsDir"`
	BaseConfig      string `mapstructure:"baseConfig"`
	DAGRunsDir      string `mapstructure:"dagRunsDir"`
	QueueDir        string `mapstructure:"queueDir"`
	ProcDir         string `mapstructure:"procDir"`
}

// uiDef holds the user interface configuration settings.
type uiDef struct {
	LogEncodingCharset    string `mapstructure:"logEncodingCharset"`
	NavbarColor           string `mapstructure:"navbarColor"`
	NavbarTitle           string `mapstructure:"navbarTitle"`
	MaxDashboardPageLimit int    `mapstructure:"maxDashboardPageLimit"`
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

// coordinatorDef represents the coordinator service configuration
type coordinatorDef struct {
	SigningKey string `mapstructure:"signingKey"`
}
