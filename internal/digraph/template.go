// Copyright (C) 2025 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package digraph

import (
	"bytes"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/davecgh/go-spew/spew"
	"mvdan.cc/sh/v3/shell"
	"mvdan.cc/sh/v3/syntax"
)

// renderTemplate replaces the template variables in the given template string
func renderTemplate(tmpl string, data map[string]any) (string, error) {
	// Create a new template instance
	templateObject := template.New("").Funcs(templateFuncs)

	// Parse the template
	parsed, err := templateObject.Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute the template
	var buf bytes.Buffer
	if err := parsed.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return strings.ReplaceAll(buf.String(), "<no value>", ""), nil
}

var templateFuncs template.FuncMap

func init() {
	// These functions are taken from the Task project (Licensed under the MIT License)
	// https://github.com/go-task/task/blob/main/internal/templater/funcs.go
	funcs := template.FuncMap{
		"OS":     func() string { return runtime.GOOS },
		"ARCH":   func() string { return runtime.GOARCH },
		"numCPU": func() int { return runtime.NumCPU() },
		"catLines": func(s string) string {
			s = strings.ReplaceAll(s, "\r\n", " ")
			return strings.ReplaceAll(s, "\n", " ")
		},
		"splitLines": func(s string) []string {
			s = strings.ReplaceAll(s, "\r\n", "\n")
			return strings.Split(s, "\n")
		},
		"fromSlash": func(path string) string {
			return filepath.FromSlash(path)
		},
		"toSlash": func(path string) string {
			return filepath.ToSlash(path)
		},
		"exeExt": func() string {
			if runtime.GOOS == "windows" {
				return ".exe"
			}
			return ""
		},
		"shellQuote": func(str string) (string, error) {
			return syntax.Quote(str, syntax.LangBash)
		},
		"splitArgs": func(s string) ([]string, error) {
			return shell.Fields(s, nil)
		},
		"joinPath": func(elem ...string) string {
			return filepath.Join(elem...)
		},
		"relPath": func(basePath, targetPath string) (string, error) {
			return filepath.Rel(basePath, targetPath)
		},
		"merge": func(base map[string]any, v ...map[string]any) map[string]any {
			cap := len(v)
			for _, m := range v {
				cap += len(m)
			}
			result := make(map[string]any, cap)
			for k, v := range base {
				result[k] = v
			}
			for _, m := range v {
				for k, v := range m {
					result[k] = v
				}
			}
			return result
		},
		"spew": func(v any) string {
			return spew.Sdump(v)
		},
	}

	// aliases
	funcs["q"] = funcs["shellQuote"]

	templateFuncs = template.FuncMap(sprig.TxtFuncMap())
	for k, v := range funcs {
		templateFuncs[k] = v
	}
}
