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

	sprig "github.com/go-task/slim-sprig/v3"

	"github.com/dagucloud/dagu/internal/cmn/eval"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/runtime"
	"github.com/dagucloud/dagu/internal/runtime/executor"
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

// blockedFuncs are removed even from the hermetic set as defense-in-depth.
// If a future slim-sprig release adds these to HermeticTxtFuncMap, we still
// block them from template steps.
var blockedFuncs = []string{
	// Environment variable access
	"env", "expandenv",
	// Network I/O
	"getHostByName",
	// Non-deterministic time
	"now", "date", "dateInZone", "date_in_zone",
	"dateModify", "date_modify", "mustDateModify", "must_date_modify",
	"ago", "duration", "durationRound",
	"unixEpoch", "toDate", "mustToDate",
	"htmlDate", "htmlDateInZone",
	// Crypto key generation
	"genPrivateKey", "derivePassword",
	"buildCustomCert", "genCA",
	"genSelfSignedCert", "genSignedCert",
	// Non-deterministic random
	"randBytes", "randString", "randNumeric",
	"randAlphaNum", "randAlpha", "randAscii", "randInt",
	"uuidv4",
}

// funcMap provides template functions for pipeline-compatible usage.
// Built from the hermetic slim-sprig base with Dagu-specific overrides.
// Functions that accept a pipeline value take it as the last argument.
var funcMap = buildFuncMap()

func buildFuncMap() template.FuncMap {
	// Start from the hermetic (no env/network/random) slim-sprig set.
	m := sprig.HermeticTxtFuncMap()

	// Defense-in-depth: remove any functions that should never be
	// available in template steps.
	for _, name := range blockedFuncs {
		delete(m, name)
	}

	// Dagu-specific overrides. These preserve pipeline-compatible argument
	// order (pipeline value as last arg) and existing behavior. Each
	// override is intentional — slim-sprig defines overlapping names with
	// different arg order or semantics.

	// split: sprig uses split(s, sep); Dagu uses split(sep, s) for pipelines.
	m["split"] = func(sep, s string) []string {
		return strings.Split(s, sep)
	}
	// join: Dagu accepts []string; also accept []any for interop with
	// sprig functions like list/uniq/sortAlpha that return []any.
	m["join"] = func(sep string, v any) string {
		if v == nil {
			return ""
		}
		switch elems := v.(type) {
		case []string:
			return strings.Join(elems, sep)
		case []any:
			strs := make([]string, len(elems))
			for i, e := range elems {
				strs[i] = fmt.Sprint(e)
			}
			return strings.Join(strs, sep)
		default:
			rv := reflect.ValueOf(v)
			if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
				strs := make([]string, rv.Len())
				for i := range strs {
					strs[i] = fmt.Sprint(rv.Index(i).Interface())
				}
				return strings.Join(strs, sep)
			}
			return fmt.Sprint(v)
		}
	}
	m["count"] = func(v any) (int, error) {
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Slice, reflect.Map, reflect.Array:
			return rv.Len(), nil
		case reflect.String:
			return rv.Len(), nil
		default:
			return 0, fmt.Errorf("count: unsupported type %T", v)
		}
	}
	// add: sprig uses add(a, b any); Dagu uses add(b, a int) for pipelines.
	m["add"] = func(b, a int) int {
		return a + b
	}
	m["empty"] = func(v any) bool {
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
	}
	m["upper"] = func(s string) string {
		return strings.ToUpper(s)
	}
	m["lower"] = func(s string) string {
		return strings.ToLower(s)
	}
	m["trim"] = func(s string) string {
		return strings.TrimSpace(s)
	}
	m["default"] = func(def, val any) any {
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
	}

	return m
}

func init() {
	executor.RegisterExecutor("template", newTemplate, validateTemplate, core.ExecutorCapabilities{
		Script: true,
		GetEvalOptions: func(_ context.Context, _ core.Step) []eval.Option {
			return []eval.Option{eval.WithNoExpansion()}
		},
	})
}
