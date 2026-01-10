package frontend

import (
	"bytes"
	"context"
	"embed"
	"io"
	"net/http"
	"path"
	"strings"
	"text/template"

	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
)

//go:embed templates/* assets/*
var assetsFS embed.FS

// templatePath is the path to the templates directory.
var templatePath = "templates/"

func (srv *Server) useTemplate(ctx context.Context, layout string, name string) func(http.ResponseWriter, any) {
	// Skip template rendering if headless
	if srv.config.Server.Headless {
		return func(w http.ResponseWriter, _ any) {
			http.Error(w, "Web UI is disabled in headless mode", http.StatusForbidden)
		}
	}

	files := append(baseTemplates(), path.Join(templatePath, layout))
	tmpl, err := template.New(name).Funcs(
		defaultFunctions(srv.funcsConfig)).ParseFS(assetsFS, files...,
	)
	if err != nil {
		panic(err)
	}

	return func(w http.ResponseWriter, data any) {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "base", data); err != nil {
			logger.Error(ctx, "Template execution failed", tag.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, &buf)
	}
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
	// OIDC configuration for builtin auth mode
	OIDCEnabled     bool
	OIDCButtonLabel string
}

// and simple utility helpers for use inside HTML templates.
func defaultFunctions(cfg funcsConfig) template.FuncMap {
	return template.FuncMap{
		"defTitle": func(ip any) string {
			v, ok := ip.(string)
			if !ok || (ok && v == "") {
				return ""
			}
			return v
		},
		"version": func() string {
			return config.Version
		},
		"navbarColor": func() string {
			return cfg.NavbarColor
		},
		"navbarTitle": func() string {
			return cfg.NavbarTitle
		},
		"basePath": func() string {
			return cfg.BasePath
		},
		"apiURL": func() string {
			return path.Join(cfg.BasePath, cfg.APIBasePath)
		},
		"tz": func() string {
			return cfg.TZ
		},
		"permissionsWriteDags": func() string {
			return convertBooleanToString(cfg.Permissions[config.PermissionWriteDAGs])
		},
		"permissionsRunDags": func() string {
			return convertBooleanToString(cfg.Permissions[config.PermissionRunDAGs])
		},
		"tzOffsetInSec": func() int {
			return cfg.TzOffsetInSec
		},
		"maxDashboardPageLimit": func() int {
			return cfg.MaxDashboardPageLimit
		},
		"remoteNodes": func() string {
			return strings.Join(cfg.RemoteNodes, ",")
		},
		"pathDAGsDir": func() string {
			return cfg.Paths.DAGsDir
		},
		"pathLogDir": func() string {
			return cfg.Paths.LogDir
		},
		"pathSuspendFlagsDir": func() string {
			return cfg.Paths.SuspendFlagsDir
		},
		"pathAdminLogsDir": func() string {
			return cfg.Paths.AdminLogsDir
		},
		"pathBaseConfig": func() string {
			return cfg.Paths.BaseConfig
		},
		"pathDAGRunsDir": func() string {
			return cfg.Paths.DAGRunsDir
		},
		"pathQueueDir": func() string {
			return cfg.Paths.QueueDir
		},
		"pathProcDir": func() string {
			return cfg.Paths.ProcDir
		},
		"pathServiceRegistryDir": func() string {
			return cfg.Paths.ServiceRegistryDir
		},
		"pathConfigFileUsed": func() string {
			return cfg.Paths.ConfigFileUsed
		},
		"pathUsersDir": func() string {
			return cfg.Paths.UsersDir
		},
		"authMode": func() string {
			return string(cfg.AuthMode)
		},
		"oidcEnabled": func() string {
			return convertBooleanToString(cfg.OIDCEnabled)
		},
		"oidcButtonLabel": func() string {
			return cfg.OIDCButtonLabel
		},
	}
}

func convertBooleanToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func baseTemplates() []string {
	var templateFiles = []string{"base.gohtml"}
	ret := make([]string, 0, len(templateFiles))
	for _, t := range templateFiles {
		ret = append(ret, path.Join(templatePath, t))
	}
	return ret
}
