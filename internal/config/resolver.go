package config

import (
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/build"
	"github.com/dagu-org/dagu/internal/fileutil"
)

// PathResolver consolidates both custom paths and XDG configuration values.
// The resulting paths will be determined based on environment variables,
// legacy configuration, or default XDG-based paths.
type PathResolver struct {
	Paths
	XDGConfig
}

// Paths holds various file system path settings used by the application.
type Paths struct {
	// ConfigDir is the primary configuration directory.
	ConfigDir string
	// DAGsDir is the directory containing DAG definitions.
	DAGsDir string
	// SuspendFlagsDir is the directory for storing flags that indicate DAG suspension.
	SuspendFlagsDir string
	// DataDir is the directory for persisting application data (e.g., history).
	DataDir string
	// LogsDir is the directory where application logs are stored.
	LogsDir string
	// AdminLogsDir is the directory where administrative logs are kept.
	AdminLogsDir string
	// BaseConfigFile is the full path to the base configuration file.
	BaseConfigFile string
	// Warnings collects any warnings encountered during path resolution.
	Warnings []string
}

// XDGConfig contains the standard XDG directories used as a fallback.
type XDGConfig struct {
	DataHome   string
	ConfigHome string
}

// NewResolver instantiates a PathResolver based on the provided application home environment variable,
// a legacy path, and an XDGConfig. It chooses the configuration directory based on these inputs.
func NewResolver(appHomeEnv, legacyPath string, xdg XDGConfig) PathResolver {
	resolver := PathResolver{XDGConfig: xdg}
	// Determine the proper configuration directory.
	resolver.resolve(appHomeEnv, legacyPath)
	return resolver
}

// resolve determines the configuration directory using the following logic:
// 1. If the environment variable (appHomeEnv) is set, use its value and assume legacy directory structure.
// 2. Else, if the legacyPath exists on disk, use it and emit a warning to update configuration paths.
// 3. Otherwise, fall back to XDG-compliant defaults.
// Note: filepath.Join ensures compatibility with Windows by using OS-specific separators.
func (r *PathResolver) resolve(appHomeEnv, legacyPath string) {
	switch {
	// Use the directory from the environment variable if available.
	case os.Getenv(appHomeEnv) != "":
		r.Paths.ConfigDir = os.Getenv(appHomeEnv)
		// Legacy paths are derived from the provided configuration directory.
		r.setLegacyPaths()
	// If the legacy path exists, warn and use it.
	case fileutil.FileExists(legacyPath):
		r.Warnings = append(r.Warnings, "Legacy path detected. Update configuration paths.")
		r.Paths.ConfigDir = legacyPath
		r.setLegacyPaths()
	// Fallback to default XDG-based paths.
	default:
		r.Paths.ConfigDir = filepath.Join(r.ConfigHome, build.Slug)
		r.setXDGPaths()
	}
}

// setXDGPaths sets the paths based on XDG environment variables.
// This approach uses the standard XDG directories (DataHome and ConfigHome)
// to organize application data, logs, and configuration files.
func (r *PathResolver) setXDGPaths() {
	r.DataDir = filepath.Join(r.DataHome, build.Slug, "history")
	r.LogsDir = filepath.Join(r.DataHome, build.Slug, "logs")
	r.BaseConfigFile = filepath.Join(r.ConfigHome, build.Slug, "base.yaml")
	r.AdminLogsDir = filepath.Join(r.DataHome, build.Slug, "logs", "admin")
	r.SuspendFlagsDir = filepath.Join(r.DataHome, build.Slug, "suspend")
	r.DAGsDir = filepath.Join(r.ConfigHome, build.Slug, "dags")
}

// setLegacyPaths sets the application paths using the legacy directory structure,
// where all subdirectories are placed within the configuration directory.
func (r *PathResolver) setLegacyPaths() {
	r.DataDir = filepath.Join(r.ConfigDir, "data")
	r.LogsDir = filepath.Join(r.ConfigDir, "logs")
	r.BaseConfigFile = filepath.Join(r.ConfigDir, "base.yaml")
	r.AdminLogsDir = filepath.Join(r.ConfigDir, "logs", "admin")
	r.SuspendFlagsDir = filepath.Join(r.ConfigDir, "suspend")
	r.DAGsDir = filepath.Join(r.ConfigDir, "dags")
}
