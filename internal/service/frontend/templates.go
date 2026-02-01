package frontend

import (
	"bytes"
	"context"
	"embed"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"text/template"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
)

//go:embed templates/* assets/*
var assetsFS embed.FS

const (
	templatePath     = "templates/"
	baseTemplateName = "base"
	baseTemplateFile = "base.gohtml"
)

func (srv *Server) useTemplate(ctx context.Context, layout, name string) func(http.ResponseWriter, any) {
	if srv.config.Server.Headless {
		return func(w http.ResponseWriter, _ any) {
			http.Error(w, "Web UI is disabled in headless mode", http.StatusForbidden)
		}
	}

	files := []string{
		path.Join(templatePath, baseTemplateFile),
		path.Join(templatePath, layout),
	}
	tmpl, err := template.New(name).Funcs(defaultFunctions(&srv.funcsConfig)).ParseFS(assetsFS, files...)
	if err != nil {
		panic(err)
	}

	return func(w http.ResponseWriter, data any) {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, baseTemplateName, data); err != nil {
			logger.Error(ctx, "Template execution failed", tag.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, &buf)
	}
}

// AgentEnabledChecker provides the agent enabled status for template rendering.
type AgentEnabledChecker interface {
	IsEnabled(ctx context.Context) bool
}

type funcsConfig struct {
	NavbarColor           string
	NavbarTitle           string
	BasePath              string
	APIBasePath           string
	TZ                    string
	TzOffsetInSec         int
	MaxDashboardPageLimit int
	RemoteNodes           []string
	Permissions           map[config.Permission]bool
	Paths                 config.PathsConfig
	AuthMode              config.AuthMode
	OIDCEnabled           bool
	OIDCButtonLabel       string
	TerminalEnabled       bool
	GitSyncEnabled        bool

	// Dynamic config sources (queried on each render)
	AgentEnabledChecker AgentEnabledChecker
}

func defaultFunctions(cfg *funcsConfig) template.FuncMap {
	boolStr := func(b bool) string { return strconv.FormatBool(b) }

	return template.FuncMap{
		"defTitle":              func(v any) string { s, _ := v.(string); return s },
		"version":               func() string { return config.Version },
		"navbarColor":           func() string { return cfg.NavbarColor },
		"navbarTitle":           func() string { return cfg.NavbarTitle },
		"basePath":              func() string { return cfg.BasePath },
		"apiURL":                func() string { return path.Join(cfg.BasePath, cfg.APIBasePath) },
		"tz":                    func() string { return cfg.TZ },
		"tzOffsetInSec":         func() int { return cfg.TzOffsetInSec },
		"maxDashboardPageLimit": func() int { return cfg.MaxDashboardPageLimit },
		"remoteNodes":           func() string { return strings.Join(cfg.RemoteNodes, ",") },
		"authMode":              func() string { return string(cfg.AuthMode) },
		"oidcButtonLabel":       func() string { return cfg.OIDCButtonLabel },

		// Permission functions
		"permissionsWriteDags": func() string { return boolStr(cfg.Permissions[config.PermissionWriteDAGs]) },
		"permissionsRunDags":   func() string { return boolStr(cfg.Permissions[config.PermissionRunDAGs]) },

		// Feature toggle functions
		"oidcEnabled":     func() string { return boolStr(cfg.OIDCEnabled) },
		"terminalEnabled": func() string { return boolStr(cfg.TerminalEnabled) },
		"gitSyncEnabled":  func() string { return boolStr(cfg.GitSyncEnabled) },
		"agentEnabled": func() string {
			if cfg.AgentEnabledChecker == nil {
				return "false"
			}
			return boolStr(cfg.AgentEnabledChecker.IsEnabled(context.Background()))
		},

		// Path configuration functions
		"pathDAGsDir":            func() string { return cfg.Paths.DAGsDir },
		"pathLogDir":             func() string { return cfg.Paths.LogDir },
		"pathSuspendFlagsDir":    func() string { return cfg.Paths.SuspendFlagsDir },
		"pathAdminLogsDir":       func() string { return cfg.Paths.AdminLogsDir },
		"pathBaseConfig":         func() string { return cfg.Paths.BaseConfig },
		"pathDAGRunsDir":         func() string { return cfg.Paths.DAGRunsDir },
		"pathQueueDir":           func() string { return cfg.Paths.QueueDir },
		"pathProcDir":            func() string { return cfg.Paths.ProcDir },
		"pathServiceRegistryDir": func() string { return cfg.Paths.ServiceRegistryDir },
		"pathConfigFileUsed":     func() string { return cfg.Paths.ConfigFileUsed },
		"pathUsersDir":           func() string { return cfg.Paths.UsersDir },
		"pathGitSyncDir":         func() string { return path.Join(cfg.Paths.DataDir, "gitsync") },
		"pathAuditLogsDir":       func() string { return path.Join(cfg.Paths.AdminLogsDir, "audit") },
	}
}
