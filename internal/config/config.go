package config

import (
	"fmt"
	"os"
	"path"
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

	// Warnings contains a list of warnings generated during the configuration loading process.
	Warnings []string
}

type Global struct {
	// Debug toggles debug mode; when true, the application may output extra logs and error details.
	Debug bool

	// LogFormat defines the output format for log messages (e.g., JSON, plain text).
	LogFormat string

	// TZ represents the timezone setting for the application (for example, "UTC" or "America/New_York").
	TZ string

	// Location represents the time location for the application based on the TZ setting.
	Location *time.Location

	// WorkDir specifies the default working directory for DAG (Directed Acyclic Graph) files.
	// If not explicitly provided, it defaults to the directory where the DAG file resides.
	WorkDir string
}

func (cfg *Global) setTimezone() error {
	if cfg.TZ != "" {
		loc, err := time.LoadLocation(cfg.TZ)
		if err != nil {
			return fmt.Errorf("failed to load timezone: %w", err)
		}
		cfg.Location = loc
		os.Setenv("TZ", cfg.TZ)
	} else {
		_, offset := time.Now().Zone()
		if offset == 0 {
			cfg.TZ = "UTC"
		} else {
			cfg.TZ = fmt.Sprintf("UTC%+d", offset/3600)
		}
		cfg.Location = time.Local
	}
	return nil
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
}

func (cfg *Server) cleanBasePath() {
	if cfg.BasePath == "" {
		return
	}

	// Clean the provided BasePath.
	cleanPath := path.Clean(cfg.BasePath)

	// Ensure the path is absolute.
	if !path.IsAbs(cleanPath) {
		cleanPath = path.Join("/", cleanPath)
	}

	// If the cleaned path is the root, reset it to an empty string.
	if cleanPath == "/" {
		cfg.BasePath = ""
	} else {
		cfg.BasePath = cleanPath
	}
}

// Auth represents the authentication configuration
type Auth struct {
	Basic AuthBasic
	Token AuthToken
}

// AuthBasic represents the basic authentication configuration
type AuthBasic struct {
	Enabled  bool
	Username string
	Password string
}

// AuthToken represents the authentication token configuration
type AuthToken struct {
	Enabled bool
	Value   string
}

// Paths represents the file system paths configuration
type PathsConfig struct {
	DAGsDir         string
	Executable      string
	LogDir          string
	DataDir         string
	SuspendFlagsDir string
	AdminLogsDir    string
	BaseConfig      string
}

type UI struct {
	LogEncodingCharset    string
	NavbarColor           string
	NavbarTitle           string
	MaxDashboardPageLimit int
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
}
