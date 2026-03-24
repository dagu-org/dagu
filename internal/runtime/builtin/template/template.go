// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package template

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"

	"github.com/dagu-org/dagu/internal/cmn/eval"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/runtime/executor"
	"github.com/go-viper/mapstructure/v2"
)

const defaultDirPermissions = 0750

var _ executor.Executor = (*templateExec)(nil)

type templateExec struct {
	stdout     io.Writer
	stderr     io.Writer
	script     string
	data       map[string]any
	outputFile string
}

type templateConfig struct {
	Data   map[string]any `mapstructure:"data"`
	Output string         `mapstructure:"output"`
}

func newTemplate(ctx context.Context, step core.Step) (executor.Executor, error) {
	var cfg templateConfig
	if step.ExecutorConfig.Config != nil {
		if err := decodeConfig(step.ExecutorConfig.Config, &cfg); err != nil {
			return nil, fmt.Errorf("template: %w", err)
		}
	}

	if step.Script == "" {
		return nil, fmt.Errorf("template: script field is required")
	}

	outputFile := cfg.Output
	if outputFile != "" && !filepath.IsAbs(outputFile) {
		env := runtime.GetEnv(ctx)
		outputFile = filepath.Join(env.WorkingDir, outputFile)
	}

	data := cfg.Data
	if data == nil {
		data = make(map[string]any)
	}

	return &templateExec{
		stdout:     os.Stdout,
		stderr:     os.Stderr,
		script:     step.Script,
		data:       data,
		outputFile: outputFile,
	}, nil
}

func (e *templateExec) SetStdout(out io.Writer) {
	e.stdout = out
}

func (e *templateExec) SetStderr(out io.Writer) {
	e.stderr = out
}

func (*templateExec) Kill(_ os.Signal) error {
	return nil
}

func (e *templateExec) Run(_ context.Context) error {
	tmpl, err := template.New("template").
		Option("missingkey=error").
		Funcs(funcMap).
		Parse(e.script)
	if err != nil {
		return fmt.Errorf("template: parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, e.data); err != nil {
		return fmt.Errorf("template: execution error: %w", err)
	}

	if e.outputFile != "" {
		return e.writeToFile(buf.Bytes())
	}

	_, err = e.stdout.Write(buf.Bytes())
	return err
}

func (e *templateExec) writeToFile(data []byte) error {
	if err := os.MkdirAll(filepath.Dir(e.outputFile), defaultDirPermissions); err != nil {
		return fmt.Errorf("template: failed to create output directory: %w", err)
	}

	tmpFile := e.outputFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return fmt.Errorf("template: failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpFile, e.outputFile); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("template: failed to rename output file: %w", err)
	}

	return nil
}

func decodeConfig(dat map[string]any, cfg *templateConfig) error {
	md, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		ErrorUnused:      false,
		Result:           cfg,
	})
	return md.Decode(dat)
}

func validateTemplate(step core.Step) error {
	if step.Script == "" {
		return fmt.Errorf("template executor requires a script field")
	}
	return nil
}

// funcMap provides template functions for pipeline-compatible usage.
// Functions that accept a pipeline value take it as the last argument.
var funcMap = template.FuncMap{
	"split": func(sep, s string) []string {
		return strings.Split(s, sep)
	},
	"join": func(sep string, elems []string) string {
		return strings.Join(elems, sep)
	},
	"count": func(v any) (int, error) {
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Slice, reflect.Map, reflect.Array:
			return rv.Len(), nil
		case reflect.String:
			return rv.Len(), nil
		default:
			return 0, fmt.Errorf("count: unsupported type %T", v)
		}
	},
	"add": func(b, a int) int {
		return a + b
	},
	"empty": func(v any) bool {
		if v == nil {
			return true
		}
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.String:
			return rv.Len() == 0
		case reflect.Slice, reflect.Map, reflect.Array:
			return rv.Len() == 0
		default:
			return rv.IsZero()
		}
	},
	"upper": func(s string) string {
		return strings.ToUpper(s)
	},
	"lower": func(s string) string {
		return strings.ToLower(s)
	},
	"trim": func(s string) string {
		return strings.TrimSpace(s)
	},
	"default": func(def, val any) any {
		if val == nil {
			return def
		}
		rv := reflect.ValueOf(val)
		switch rv.Kind() {
		case reflect.String:
			if rv.Len() == 0 {
				return def
			}
		case reflect.Slice, reflect.Map, reflect.Array:
			if rv.Len() == 0 {
				return def
			}
		default:
			if rv.IsZero() {
				return def
			}
		}
		return val
	},
}

func init() {
	executor.RegisterExecutor("template", newTemplate, validateTemplate, core.ExecutorCapabilities{
		Script: true,
		GetEvalOptions: func(_ context.Context, _ core.Step) []eval.Option {
			return []eval.Option{eval.WithNoExpansion()}
		},
	})
}
