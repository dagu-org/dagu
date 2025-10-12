package config

import (
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/common/fileutil"
)

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

// ResolvePaths determines application paths based on the provided application home environment variable,
// a legacy path, and an XDGConfig. It chooses the configuration directory based on these inputs.
//
// Resolution logic:
// 1. If the environment variable (appHomeEnv) is set, use its value and assume legacy directory structure.
// 2. Else, if the legacyPath exists on disk, use it and emit a warning to update configuration paths.
// 3. Otherwise, fall back to XDG-compliant defaults.
func ResolvePaths(appHomeEnv, legacyPath string, xdg XDGConfig) Paths {
	switch {
	// Use the directory from the environment variable if available.
	case os.Getenv(appHomeEnv) != "":
		configDir := os.Getenv(appHomeEnv)
		return setLegacyPaths(configDir)
	// If the legacy path exists, warn and use it.
	case fileutil.FileExists(legacyPath):
		return setLegacyPaths(legacyPath)
	// Fallback to default XDG-based paths.
	default:
		configDir := filepath.Join(xdg.ConfigHome, AppSlug)
		return setXDGPaths(xdg, configDir)
	}
}

// setXDGPaths sets the paths based on XDG environment variables.
// This approach uses the standard XDG directories (DataHome and ConfigHome)
// to organize application data, logs, and configuration files.
func setXDGPaths(xdg XDGConfig, configDir string) Paths {
	return Paths{
		ConfigDir:       configDir,
		DataDir:         filepath.Join(xdg.DataHome, AppSlug, "data"),
		LogsDir:         filepath.Join(xdg.DataHome, AppSlug, "logs"),
		BaseConfigFile:  filepath.Join(xdg.ConfigHome, AppSlug, "base.yaml"),
		AdminLogsDir:    filepath.Join(xdg.DataHome, AppSlug, "logs", "admin"),
		SuspendFlagsDir: filepath.Join(xdg.DataHome, AppSlug, "suspend"),
		DAGsDir:         filepath.Join(xdg.ConfigHome, AppSlug, "dags"),
	}
}

// setLegacyPaths sets the application paths using the legacy directory structure,
// where all subdirectories are placed within the configuration directory.
func setLegacyPaths(configDir string) Paths {
	return Paths{
		ConfigDir:       configDir,
		DataDir:         filepath.Join(configDir, "data"),
		LogsDir:         filepath.Join(configDir, "logs"),
		BaseConfigFile:  filepath.Join(configDir, "base.yaml"),
		AdminLogsDir:    filepath.Join(configDir, "logs", "admin"),
		SuspendFlagsDir: filepath.Join(configDir, "suspend"),
		DAGsDir:         filepath.Join(configDir, "dags"),
	}
}
