package config

import (
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/build"
	"github.com/dagu-org/dagu/internal/fileutil"
)

type PathResolver struct {
	Paths
	XDGConfig
}

type Paths struct {
	ConfigDir       string
	DAGsDir         string
	SuspendFlagsDir string
	DataDir         string
	LogsDir         string
	AdminLogsDir    string
	BaseConfigFile  string
}

type XDGConfig struct {
	DataHome   string
	ConfigHome string
}

func newResolver(appHomeEnv, legacyPath string, xdg XDGConfig) PathResolver {
	resolver := PathResolver{XDGConfig: xdg}
	resolver.resolve(appHomeEnv, legacyPath)

	return resolver
}

func (r *PathResolver) resolve(appHomeEnv, legacyPath string) {
	switch {
	case os.Getenv(appHomeEnv) != "":
		r.Paths.ConfigDir = os.Getenv(appHomeEnv)
		r.setLegacyPaths()
	case fileutil.FileExists(legacyPath):
		r.Paths.ConfigDir = legacyPath
		r.setLegacyPaths()
	default:
		r.Paths.ConfigDir = filepath.Join(r.ConfigHome, build.Slug)
		r.setXDGPaths()
	}
}

func (r *PathResolver) setXDGPaths() {
	r.DataDir = filepath.Join(r.DataHome, build.Slug, "history")
	r.LogsDir = filepath.Join(r.DataHome, build.Slug, "logs")
	r.BaseConfigFile = filepath.Join(r.ConfigHome, build.Slug, "base.yaml")
	r.AdminLogsDir = filepath.Join(r.DataHome, build.Slug, "logs", "admin")
	r.SuspendFlagsDir = filepath.Join(r.DataHome, build.Slug, "suspend")
	r.DAGsDir = filepath.Join(r.ConfigHome, build.Slug, "dags")
}

func (r *PathResolver) setLegacyPaths() {
	r.DataDir = filepath.Join(r.ConfigDir, "data")
	r.LogsDir = filepath.Join(r.ConfigDir, "logs")
	r.BaseConfigFile = filepath.Join(r.ConfigDir, "base.yaml")
	r.AdminLogsDir = filepath.Join(r.ConfigDir, "logs", "admin")
	r.SuspendFlagsDir = filepath.Join(r.ConfigDir, "suspend")
	r.DAGsDir = filepath.Join(r.ConfigDir, "dags")
}
