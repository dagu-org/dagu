package frontend

import (
	"bytes"
	"context"
	"embed"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	_ "embed"

	"github.com/dagu-org/dagu/internal/build"
	"github.com/dagu-org/dagu/internal/logger"
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

	files := append(baseTemplates(), filepath.Join(templatePath, layout))
	tmpl, err := template.New(name).Funcs(
		defaultFunctions(srv.funcsConfig)).ParseFS(assetsFS, files...,
	)
	if err != nil {
		panic(err)
	}

	return func(w http.ResponseWriter, data any) {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "base", data); err != nil {
			logger.Error(ctx, "Template execution failed", "err", err)
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
	MaxDashboardPageLimit int
	RemoteNodes           []string
}

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
			return build.Version
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
		"maxDashboardPageLimit": func() int {
			return cfg.MaxDashboardPageLimit
		},
		"remoteNodes": func() string {
			return strings.Join(cfg.RemoteNodes, ",")
		},
	}
}

func baseTemplates() []string {
	var templateFiles = []string{"base.gohtml"}
	ret := make([]string, 0, len(templateFiles))
	for _, t := range templateFiles {
		ret = append(ret, filepath.Join(templatePath, t))
	}
	return ret
}
