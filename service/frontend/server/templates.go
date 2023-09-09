package server

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"path"
	"text/template"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/constants"
)

var (
	// templatePath is the path to the templates directory.
	templatePath = "templates/"
)

func (svr *Server) useTemplate(layout string, name string) func(http.ResponseWriter, interface{}) {
	files := append(baseTemplates(), path.Join(templatePath, layout))
	tmpl, err := template.New(name).Funcs(defaultFunctions()).ParseFS(svr.assets, files...)
	if err != nil {
		panic(err)
	}

	return func(w http.ResponseWriter, data interface{}) {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "base", data); err != nil {
			log.Printf("ERR: %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, &buf)
	}
}

func defaultFunctions() template.FuncMap {
	cfg := config.Get()

	return template.FuncMap{
		"defTitle": func(ip interface{}) string {
			v, ok := ip.(string)
			if !ok || (ok && v == "") {
				return ""
			}
			return v
		},
		"version": func() string {
			return constants.Version
		},
		"navbarColor": func() string {
			return cfg.NavbarColor
		},
		"navbarTitle": func() string {
			return cfg.NavbarTitle
		},
		"apiURL": func() string {
			return cfg.GetAPIBaseURL()
		},
	}
}

func baseTemplates() []string {
	var templateFiles = []string{"base.gohtml"}
	ret := make([]string, 0, len(templateFiles))
	for _, t := range templateFiles {
		ret = append(ret, path.Join(templatePath, t))
	}
	return ret
}
