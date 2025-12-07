package config

import (
	"fmt"
	"time"
)

// Config holds the overall configuration for the application.
type Config struct {
	// Global contains global configuration settings.
	Global Global

	// Server contains the API server configuration.
	Server Server

	// Paths holds various filesystem path configurations used throughout the application.
	Paths PathsConfig

	// UI contains settings specific to the application's user interface.
	UI UI

	// Queues contains global queue configuration settings.
	Queues Queues

	// Coordinator defines the coordinator service configuration.
	Coordinator Coordinator

	// Worker defines the worker configuration.
	Worker Worker

	// Scheduler defines the scheduler configuration.
	Scheduler Scheduler

	// Warnings contains a list of warnings generated during the configuration loading process.
	Warnings []string

	// Monitoring contains configuration for system monitoring.
	Monitoring MonitoringConfig
}

// MonitoringConfig holds the configuration for system monitoring.
// Memory estimation: Each metric point is ~16 bytes. With 4 metrics collected
// every 5 seconds for 24 hours, that's ~1.1MB of memory usage.
// Formula: 4 metrics × (retention / interval) × 16 bytes
type MonitoringConfig struct {
	// Retention specifies how long to keep system resource history.
	Retention time.Duration
	// Interval specifies how often to collect resource metrics.
	Interval time.Duration
}

type Global struct {
	// Debug toggles debug mode; when true, the application may output extra logs and error details.
	Debug bool

	// LogFormat defines the output format for log messages (e.g., JSON, plain text).
	LogFormat string

	// TZ represents the timezone setting for the application (for example, "UTC" or "America/New_York").
	TZ string

	// TzOffsetInSec is the offset from UTC in seconds.
	TzOffsetInSec int

	// Location represents the time location for the application based on the TZ setting.
	Location *time.Location

	// DefaultShell specifies the default shell to use for command execution.
	// If not provided, platform-specific defaults are used (PowerShell on Windows, $SHELL on Unix).
	DefaultShell string

	// SkipExamples disables the automatic creation of example DAGs when the DAGs directory is empty.
	SkipExamples bool

	// Peer contains configuration for peer connections over gRPC.
	Peer Peer

	// BaseEnv holds base environment variables to be used for child processes.
	BaseEnv BaseEnv
}

// Server contains the API server configuration
type Server struct {
	// Host defines the hostname or IP address on which the application will run.
	Host string

	// Port specifies the network port for incoming connections.
	Port int

	// BasePath is the root URL path from which the application is served.
	// This is useful when hosting the app behind a reverse proxy under a subpath.
	BasePath string

	// APIBasePath sets the base path for all API endpoints provided by the application.
	APIBasePath string

	// Headless determines if the application should run without a graphical user interface.
	// Useful for automated or headless server environments.
	Headless bool

	// LatestStatusToday indicates whether the application should display only the most recent status for the current day.
	LatestStatusToday bool

	// TLS contains configuration details for enabling TLS/SSL encryption,
	// such as certificate and key file paths.
	TLS *TLSConfig

	// Auth contains authentication settings (such as credentials or tokens) needed to secure the application.
	Auth Auth

	// RemoteNodes holds a list of configurations for connecting to remote nodes.
	// This enables the management of DAGs on external servers.
	RemoteNodes []RemoteNode

	// Permissions defines the permissions allowed in the UI and API.
	Permissions map[Permission]bool

	// StrictValidation enables strict validation of API requests.
	StrictValidation bool
}

// Permission represents a permission string used in the application.
type Permission string

const (
	PermissionWriteDAGs Permission = "write_dags"
	PermissionRunDAGs   Permission = "run_dags"
)

// Auth represents the authentication configuration
type Auth struct {
	Basic AuthBasic
	Token AuthToken
	OIDC  AuthOIDC
}

// AuthBasic represents the basic authentication configuration
type AuthBasic struct {
	Username string
	Password string
}

// AuthToken represents the authentication token configuration
type AuthToken struct {
	Value string
}

type AuthOIDC struct {
	ClientId     string   //id from the authorization service (OIDC provider)
	ClientSecret string   //secret from the authorization service (OIDC provider)
	ClientUrl    string   //your website's/service's URL for example: "http://localhost:8081/" or "https://mydomain.com/
	Issuer       string   //the URL identifier for the authorization service. for example: "https://accounts.google.com" - try adding "/.well-known/openid-configuration" to the path to make sure it's correct
	Scopes       []string //OAuth scopes. If you're unsure go with: []string{oidc.ScopeOpenID, "profile", "email"}
	Whitelist    []string //OAuth User whitelist ref userinfo.email https://github.com/coreos/go-oidc/blob/v2/oidc.go#L199
}

// Paths represents the file system paths configuration
type PathsConfig struct {
	DAGsDir            string
	Executable         string
	LogDir             string
	DataDir            string
	SuspendFlagsDir    string
	AdminLogsDir       string
	BaseConfig         string
	DAGRunsDir         string
	QueueDir           string
	ProcDir            string
	ServiceRegistryDir string // Directory for service registry files
	ConfigFileUsed     string // Path to the configuration file used to load settings
}

type UI struct {
	LogEncodingCharset    string
	NavbarColor           string
	NavbarTitle           string
	MaxDashboardPageLimit int
	DAGs                  DAGsConfig
}

type DAGsConfig struct {
	SortField string
	SortOrder string
}

// RemoteNode represents a remote node configuration
type RemoteNode struct {
	Name              string
	APIBaseURL        string
	IsBasicAuth       bool
	BasicAuthUsername string
	BasicAuthPassword string
	IsAuthToken       bool
	AuthToken         string
	SkipTLSVerify     bool
}

// TLSConfig represents TLS configuration
type TLSConfig struct {
	CertFile string
	KeyFile  string
	CAFile   string
}

// Queues represents the global queue configuration
type Queues struct {
	Enabled bool
	Config  []QueueConfig
}

// QueueConfig represents individual queue configuration
type QueueConfig struct {
	Name          string
	MaxActiveRuns int
}

// Coordinator represents the coordinator service configuration
type Coordinator struct {
	ID        string // Coordinator instance ID (default: hostname@port)
	Host      string // gRPC server bind address (e.g., 0.0.0.0, 127.0.0.1)
	Advertise string // Address to advertise in service registry (default: auto-detected hostname)
	Port      int    // gRPC server port number
}

// Worker represents the worker configuration
type Worker struct {
	ID            string            // Worker instance ID (default: hostname@PID)
	MaxActiveRuns int               // Maximum number of active runs (default: 100)
	Labels        map[string]string // Worker labels for capability matching
}

// Scheduler represents the scheduler configuration
type Scheduler struct {
	// Port is the port on which the scheduler's health check server listens.
	Port int // Health check server port (default: 8090)

	// SchedulerLockStaleThreshold is the time after which a scheduler lock is considered stale.
	// Default is 30 seconds.
	LockStaleThreshold time.Duration

	// SchedulerLockRetryInterval is the interval between lock acquisition attempts.
	// Default is 5 seconds.
	LockRetryInterval time.Duration

	// ZombieDetectionInterval is the interval between checks for zombie DAG runs.
	// A zombie DAG run is one marked as running but whose process is no longer alive.
	// Set to 0 to disable zombie detection. Default is 45 seconds.
	ZombieDetectionInterval time.Duration
}

// Peer holds the certificate and TLS configuration for peer connections over gRPC.
type Peer struct {
	// CertFile is the path to the server's TLS certificate file.
	CertFile string

	// KeyFile is the path to the server's TLS key file.
	KeyFile string

	// ClientCaFile is the path to the CA certificate file used for client verification.
	ClientCaFile string

	// SkipTLSVerify indicates whether to skip TLS certificate verification.
	SkipTLSVerify bool

	// Insecure indicates whether to use insecure connection (h2c) instead of TLS.
	Insecure bool
}

// Validate performs basic validation on the configuration to ensure required fields are set
// and that numerical values fall within acceptable ranges.
func (c *Config) Validate() error {
	if c.Server.Port < 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", c.Server.Port)
	}

	if c.Server.TLS != nil {
		if c.Server.TLS.CertFile == "" || c.Server.TLS.KeyFile == "" {
			return fmt.Errorf("TLS configuration incomplete: both cert and key files are required")
		}
	}

	if c.UI.MaxDashboardPageLimit < 1 {
		return fmt.Errorf("invalid max dashboard page limit: %d", c.UI.MaxDashboardPageLimit)
	}

	return nil
}
