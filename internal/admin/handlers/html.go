package handlers

import (
	"bytes"
	"embed"
	"io"
	"log"
	"net/http"
	"path"
	"text/template"

	"github.com/yohamta/dagu/internal/constants"
	"github.com/yohamta/dagu/internal/settings"
)

var defaultFuncs = template.FuncMap{
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
		c, _ := settings.Get(settings.SETTING__ADMIN_NAVBAR_COLOR)
		return c
	},
	"navbarTitle": func() string {
		val, _ := settings.Get(settings.SETTING__ADMIN_NAVBAR_TITLE)
		return val
	},
}

//go:embed web/templates/* web/assets/*
var assets embed.FS
var templatePath = "web/templates/"

func useTemplate(layout string, name string) func(http.ResponseWriter, interface{}) {
	files := append(baseTemplates(), path.Join(templatePath, layout))
	tmpl, err := template.New(name).Funcs(defaultFuncs).ParseFS(assets, files...)
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
		io.Copy(w, &buf)
	}
}

func baseTemplates() []string {
	var templateFiles = []string{
		"base.gohtml",
	}
	ret := []string{}
	for _, t := range templateFiles {
		ret = append(ret, path.Join(templatePath, t))
	}
	return ret
}
