package config

import (
	"fmt"
	"slices"
	"time"
)

// Config holds the overall configuration for the application.
type Config struct {
	// Core contains global configuration settings.
	Core Core

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

	// Cache defines the cache mode preset (low, normal, high).
	Cache CacheMode

	// GitSync contains configuration for Git synchronization.
	GitSync GitSyncConfig

	// Tunnel contains configuration for tunnel services (Cloudflare/Tailscale).
	Tunnel TunnelConfig
}

// GitSyncConfig holds the configuration for Git sync functionality.
type GitSyncConfig struct {
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
	Auth GitSyncAuthConfig

	// AutoSync contains auto-sync configuration.
	AutoSync GitSyncAutoSyncConfig

	// PushEnabled indicates whether pushing changes is allowed.
	PushEnabled bool

	// Commit contains commit configuration.
	Commit GitSyncCommitConfig
}

// GitSyncAuthConfig holds authentication configuration for Git operations.
type GitSyncAuthConfig struct {
	// Type is the authentication type: "token" or "ssh".
	Type string

	// Token is the personal access token for HTTPS authentication.
	Token string

	// SSHKeyPath is the path to the SSH private key file.
	SSHKeyPath string

	// SSHPassphrase is the passphrase for the SSH key (optional).
	SSHPassphrase string
}

// GitSyncAutoSyncConfig holds configuration for automatic synchronization.
type GitSyncAutoSyncConfig struct {
	// Enabled indicates whether auto-sync is enabled.
	Enabled bool

	// OnStartup indicates whether to sync on server startup.
	OnStartup bool

	// Interval is the sync interval in seconds.
	// 0 means auto-sync is disabled (pull on startup only).
	Interval int
}

// GitSyncCommitConfig holds configuration for Git commits.
type GitSyncCommitConfig struct {
	// AuthorName is the name to use for commits.
	// Defaults to "Dagu" if not specified.
	AuthorName string

	// AuthorEmail is the email to use for commits.
	// Defaults to "dagu@localhost" if not specified.
	AuthorEmail string
}

// TunnelConfig holds the configuration for tunnel services.
type TunnelConfig struct {
	// Enabled indicates whether tunneling is enabled.
	Enabled bool

	// Provider specifies which tunnel provider to use: "cloudflare" or "tailscale".
	Provider string

	// Cloudflare contains Cloudflare Tunnel configuration.
	Cloudflare CloudflareTunnelConfig

	// Tailscale contains Tailscale configuration.
	Tailscale TailscaleTunnelConfig

	// AllowTerminal allows terminal access via tunnel (default: false for security).
	AllowTerminal bool

	// AllowedIPs is an IP allowlist (empty = allow all).
	AllowedIPs []string

	// RateLimiting contains rate limiting configuration for auth endpoints.
	RateLimiting TunnelRateLimitConfig
}

// CloudflareTunnelConfig holds Cloudflare Tunnel settings.
type CloudflareTunnelConfig struct {
	// Token is the Cloudflare Tunnel token (required for named tunnels).
	// Get this from Cloudflare Dashboard → Zero Trust → Tunnels.
	Token string

	// Hostname is the custom hostname for the tunnel.
	// If empty, uses the default cfargotunnel.com subdomain.
	Hostname string
}

// TailscaleTunnelConfig holds Tailscale settings.
type TailscaleTunnelConfig struct {
	// AuthKey is the Tailscale auth key for headless authentication.
	// If empty, interactive login via URL will be required.
	AuthKey string

	// Hostname is the machine name in the tailnet (default: "dagu").
	Hostname string

	// Funnel enables Tailscale Funnel for public internet access.
	// When false, the server is only accessible within the tailnet.
	Funnel bool

	// HTTPS enables HTTPS for tailnet-only access.
	// Requires enabling HTTPS certificates in the Tailscale admin panel.
	// When false, uses plain HTTP (still secure via WireGuard encryption).
	HTTPS bool

	// StateDir is the directory for Tailscale state storage.
	// Default: $DAGU_HOME/tailscale
	StateDir string
}

// TunnelRateLimitConfig holds rate limiting configuration.
type TunnelRateLimitConfig struct {
	// Enabled indicates whether rate limiting is enabled.
	Enabled bool

	// LoginAttempts is the maximum login attempts per window.
	LoginAttempts int

	// WindowSeconds is the time window in seconds.
	WindowSeconds int

	// BlockDurationSeconds is the block duration after exceeding limit.
	BlockDurationSeconds int
}

// TunnelProvider constants
const (
	TunnelProviderCloudflare = "cloudflare"
	TunnelProviderTailscale  = "tailscale"
)

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

// Core contains global configuration settings.
type Core struct {
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

	// Metrics controls access to the /api/v2/metrics endpoint.
	// "private" (default) requires authentication, "public" allows unauthenticated access.
	Metrics MetricsAccess

	// Terminal contains configuration for the web-based terminal feature.
	Terminal TerminalConfig

	// Audit contains configuration for the audit logging feature.
	Audit AuditConfig
}

// TerminalConfig contains configuration for the web-based terminal feature.
type TerminalConfig struct {
	// Enabled determines if the terminal feature is available.
	// Default: false
	// Env: DAGU_TERMINAL_ENABLED
	Enabled bool
}

// AuditConfig contains configuration for the audit logging feature.
type AuditConfig struct {
	// Enabled determines if audit logging is active.
	// Default: true
	// Env: DAGU_AUDIT_ENABLED
	Enabled bool
}

// Permission represents a permission string used in the application.
type Permission string

const (
	PermissionWriteDAGs Permission = "write_dags"
	PermissionRunDAGs   Permission = "run_dags"
)

// AuthMode represents the authentication mode.
type AuthMode string

const (
	// AuthModeNone disables authentication.
	AuthModeNone AuthMode = "none"
	// AuthModeBuiltin enables builtin user management with RBAC.
	AuthModeBuiltin AuthMode = "builtin"
	// AuthModeOIDC enables OIDC authentication.
	AuthModeOIDC AuthMode = "oidc"
)

// MetricsAccess represents the access mode for the metrics endpoint.
type MetricsAccess string

const (
	// MetricsAccessPrivate requires authentication to access the metrics endpoint.
	MetricsAccessPrivate MetricsAccess = "private"
	// MetricsAccessPublic allows unauthenticated access to the metrics endpoint.
	MetricsAccessPublic MetricsAccess = "public"
)

// Auth represents the authentication configuration
type Auth struct {
	Mode    AuthMode
	Basic   AuthBasic
	Token   AuthToken
	OIDC    AuthOIDC
	Builtin AuthBuiltin
}

// AuthBuiltin represents the builtin authentication configuration
type AuthBuiltin struct {
	Admin AdminConfig
	Token TokenConfig
}

// OIDCRoleMapping defines how OIDC claims are mapped to Dagu roles
type OIDCRoleMapping struct {
	// DefaultRole is the role assigned to new OIDC users when no mapping matches (default: "viewer")
	DefaultRole string
	// GroupsClaim specifies the claim name containing groups (default: "groups")
	GroupsClaim string
	// GroupMappings maps IdP group names to Dagu roles
	GroupMappings map[string]string
	// RoleAttributePath is a jq expression to extract role from claims
	RoleAttributePath string
	// RoleAttributeStrict denies login when no valid role is found
	RoleAttributeStrict bool
	// SkipOrgRoleSync skips role sync on subsequent logins
	SkipOrgRoleSync bool
}

// AdminConfig represents the initial admin user configuration
type AdminConfig struct {
	Username string
	Password string
}

// TokenConfig represents the JWT token configuration
type TokenConfig struct {
	Secret string
	TTL    time.Duration
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

// AuthOIDC represents the OIDC authentication configuration.
// Core fields (ClientId, ClientSecret, etc.) are used by both standalone OIDC mode
// and builtin auth mode with OIDC login. Builtin-specific fields (AutoSignup,
// DefaultRole, etc.) are only used when auth.mode=builtin.
// OIDC is automatically enabled when all required fields are configured.
type AuthOIDC struct {
	// Core OIDC fields (used by both standalone and builtin modes)
	ClientId     string   // OIDC client ID from the authorization service
	ClientSecret string   // OIDC client secret from the authorization service
	ClientUrl    string   // Application URL for callback (e.g., "https://mydomain.com")
	Issuer       string   // OIDC provider URL (e.g., "https://accounts.google.com")
	Scopes       []string // OAuth scopes (default: openid, profile, email)
	Whitelist    []string // Specific email addresses always allowed

	// Builtin-specific fields (only used when auth.mode=builtin)
	AutoSignup     bool            // Auto-create users on first login (default: true)
	AllowedDomains []string        // Email domain whitelist
	ButtonLabel    string          // Login button text (default: "Login with SSO")
	RoleMapping    OIDCRoleMapping // Role mapping configuration
}

// IsConfigured returns true if all required OIDC fields are set.
// When true, OIDC login is automatically enabled under builtin auth mode.
func (o AuthOIDC) IsConfigured() bool {
	return o.ClientId != "" && o.ClientSecret != "" && o.ClientUrl != "" && o.Issuer != ""
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
	UsersDir           string // Directory for user data (builtin auth)
	APIKeysDir         string // Directory for API key data (builtin auth)
	WebhooksDir        string // Directory for webhook data (builtin auth)
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
	Coordinators  []string          // Coordinator addresses for static discovery (host:port)

	// PostgresPool holds connection pool settings for shared-nothing mode.
	// When multiple DAGs run concurrently in a worker, they share this pool.
	PostgresPool PostgresPoolConfig
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

// PostgresPoolConfig holds PostgreSQL connection pool settings for workers.
// Used in shared-nothing worker mode to prevent connection exhaustion
// when multiple DAGs run concurrently in a single worker process.
type PostgresPoolConfig struct {
	// MaxOpenConns is the maximum total open connections across ALL PostgreSQL DSNs.
	// This is the hard limit shared across all database connections.
	// Default: 25
	MaxOpenConns int

	// MaxIdleConns is the maximum number of idle connections per DSN.
	// Default: 5
	MaxIdleConns int

	// ConnMaxLifetime is the maximum lifetime of a connection in seconds.
	// Default: 300 (5 minutes)
	ConnMaxLifetime int

	// ConnMaxIdleTime is the maximum idle time for a connection in seconds.
	// Default: 60 (1 minute)
	ConnMaxIdleTime int
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

	// MaxRetries is the maximum number of retry attempts for coordinator connections.
	// Uses exponential backoff: interval * 2^attempt, capped at 30s.
	// Default is 10 for better resilience during startup.
	MaxRetries int

	// RetryInterval is the base interval between retry attempts.
	// Default is 1 second.
	RetryInterval time.Duration
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

	// Validate auth mode
	switch c.Server.Auth.Mode {
	case AuthModeNone, AuthModeBuiltin, AuthModeOIDC:
		// Valid modes
	default:
		return fmt.Errorf("invalid auth mode: %q (must be one of: none, builtin, oidc)", c.Server.Auth.Mode)
	}

	// Validate builtin auth configuration
	if err := c.validateBuiltinAuth(); err != nil {
		return err
	}

	// Validate Git sync configuration
	if err := c.validateGitSync(); err != nil {
		return err
	}

	// Validate tunnel configuration
	if err := c.validateTunnel(); err != nil {
		return err
	}

	return nil
}

// validateBuiltinAuth validates the builtin authentication configuration.
func (c *Config) validateBuiltinAuth() error {
	if c.Server.Auth.Mode != AuthModeBuiltin {
		return nil
	}

	// When builtin auth is enabled, users directory must be set
	if c.Paths.UsersDir == "" {
		return fmt.Errorf("builtin auth requires paths.usersDir to be set")
	}

	// Admin username must be set (has default, but check anyway)
	if c.Server.Auth.Builtin.Admin.Username == "" {
		return fmt.Errorf("builtin auth requires admin username to be set")
	}

	// Token secret must be set for JWT signing
	if c.Server.Auth.Builtin.Token.Secret == "" {
		return fmt.Errorf("builtin auth requires token secret to be set (auth.builtin.token.secret or AUTH_TOKEN_SECRET env var)")
	}

	// Token TTL must be positive
	if c.Server.Auth.Builtin.Token.TTL <= 0 {
		return fmt.Errorf("builtin auth requires a positive token TTL")
	}

	// Validate OIDC configuration if configured under builtin auth
	if c.Server.Auth.OIDC.IsConfigured() {
		if err := c.validateOIDCForBuiltin(); err != nil {
			return err
		}
	}

	return nil
}

// validateOIDCForBuiltin validates the OIDC configuration when used under builtin auth mode.
func (c *Config) validateOIDCForBuiltin() error {
	oidc := c.Server.Auth.OIDC

	// Required fields when OIDC is enabled
	if oidc.ClientId == "" {
		return fmt.Errorf("OIDC requires clientId to be set (auth.oidc.clientId or AUTH_OIDC_CLIENT_ID)")
	}
	if oidc.ClientSecret == "" {
		return fmt.Errorf("OIDC requires clientSecret to be set (auth.oidc.clientSecret or AUTH_OIDC_CLIENT_SECRET)")
	}
	if oidc.ClientUrl == "" {
		return fmt.Errorf("OIDC requires clientUrl to be set (auth.oidc.clientUrl or AUTH_OIDC_CLIENT_URL)")
	}
	if oidc.Issuer == "" {
		return fmt.Errorf("OIDC requires issuer to be set (auth.oidc.issuer or AUTH_OIDC_ISSUER)")
	}

	// Validate defaultRole is a valid role
	validRoles := map[string]bool{
		"admin":    true,
		"manager":  true,
		"operator": true,
		"viewer":   true,
	}
	if !validRoles[oidc.RoleMapping.DefaultRole] {
		return fmt.Errorf("OIDC roleMapping.defaultRole must be one of: admin, manager, operator, viewer (got: %q)", oidc.RoleMapping.DefaultRole)
	}

	// Check if email scope is included
	hasEmailScope := slices.Contains(oidc.Scopes, "email")

	// Error if access control features require email but scope is missing
	if !hasEmailScope {
		if len(oidc.Whitelist) > 0 || len(oidc.AllowedDomains) > 0 {
			return fmt.Errorf("OIDC scopes must include 'email' when whitelist or allowedDomains is configured")
		}
		// Just warn if no access control is configured
		c.Warnings = append(c.Warnings,
			"OIDC scopes do not include 'email'; access control features will not work if added later")
	}

	return nil
}

// validateGitSync validates the Git sync configuration.
func (c *Config) validateGitSync() error {
	if !c.GitSync.Enabled {
		return nil
	}

	// Repository is required when enabled
	if c.GitSync.Repository == "" {
		return fmt.Errorf("git sync requires repository to be set (gitSync.repository)")
	}

	// Branch is required when enabled
	if c.GitSync.Branch == "" {
		return fmt.Errorf("git sync requires branch to be set (gitSync.branch)")
	}

	// Validate auth type
	switch c.GitSync.Auth.Type {
	case "token", "ssh":
		// Valid auth types
	case "":
		// Empty is allowed (defaults to token)
	default:
		return fmt.Errorf("git sync auth type must be 'token' or 'ssh' (got: %q)", c.GitSync.Auth.Type)
	}

	// Validate SSH auth requires key path
	if c.GitSync.Auth.Type == "ssh" && c.GitSync.Auth.SSHKeyPath == "" {
		return fmt.Errorf("git sync SSH auth requires sshKeyPath to be set")
	}

	// Validate auto sync interval is non-negative
	if c.GitSync.AutoSync.Interval < 0 {
		return fmt.Errorf("git sync autoSync.interval must be non-negative (got: %d)", c.GitSync.AutoSync.Interval)
	}

	return nil
}

// validateTunnel validates the tunnel configuration.
func (c *Config) validateTunnel() error {
	if !c.Tunnel.Enabled {
		return nil
	}

	// Validate provider
	switch c.Tunnel.Provider {
	case TunnelProviderCloudflare:
		// Cloudflare requires a token for named tunnels
		if c.Tunnel.Cloudflare.Token == "" {
			return fmt.Errorf("cloudflare tunnel requires token to be set (tunnel.cloudflare.token)")
		}
	case TunnelProviderTailscale:
		// Tailscale doesn't strictly require config, but hostname is recommended
	case "":
		return fmt.Errorf("tunnel provider must be set (tunnel.provider: cloudflare or tailscale)")
	default:
		return fmt.Errorf("invalid tunnel provider: %q (must be 'cloudflare' or 'tailscale')", c.Tunnel.Provider)
	}

	// Check if tunnel is public (Cloudflare or Tailscale with Funnel)
	isPublic := c.Tunnel.Provider == TunnelProviderCloudflare ||
		(c.Tunnel.Provider == TunnelProviderTailscale && c.Tunnel.Tailscale.Funnel)

	// SECURITY: Public tunnels REQUIRE authentication
	if isPublic && c.Server.Auth.Mode == AuthModeNone {
		return fmt.Errorf(
			"tunnel with public access requires authentication; "+
				"set server.auth.mode=builtin or disable tailscale funnel for private access",
		)
	}

	// Validate rate limiting config
	if c.Tunnel.RateLimiting.Enabled {
		if c.Tunnel.RateLimiting.LoginAttempts <= 0 {
			return fmt.Errorf("tunnel rate limiting loginAttempts must be positive")
		}
		if c.Tunnel.RateLimiting.WindowSeconds <= 0 {
			return fmt.Errorf("tunnel rate limiting windowSeconds must be positive")
		}
		if c.Tunnel.RateLimiting.BlockDurationSeconds <= 0 {
			return fmt.Errorf("tunnel rate limiting blockDurationSeconds must be positive")
		}
	}

	return nil
}

// IsTunnelPublic returns true if the tunnel exposes the service to the public internet.
func (c *Config) IsTunnelPublic() bool {
	if !c.Tunnel.Enabled {
		return false
	}
	return c.Tunnel.Provider == TunnelProviderCloudflare ||
		(c.Tunnel.Provider == TunnelProviderTailscale && c.Tunnel.Tailscale.Funnel)
}
