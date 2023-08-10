package handlers

import (
	"bytes"
	"embed"
	"io"
	"log"
	"net/http"
	"path"
	"text/template"

	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/constants"
)

func defaultFuncs() template.FuncMap {
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
	}
}

//go:embed templates/* assets/*
var assetsFS embed.FS
var templatePath = "templates/"

func useTemplate(layout string, name string) func(http.ResponseWriter, interface{}) {
	files := append(baseTemplates(), path.Join(templatePath, layout))
	tmpl, err := template.New(name).Funcs(defaultFuncs()).ParseFS(assetsFS, files...)
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

func baseTemplates() []string {
	var templateFiles = []string{"base.gohtml"}
	ret := []string{}
	for _, t := range templateFiles {
		ret = append(ret, path.Join(templatePath, t))
	}
	return ret
}
