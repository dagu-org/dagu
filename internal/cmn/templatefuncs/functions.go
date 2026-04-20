// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package templatefuncs

import (
	"fmt"
	"reflect"
	"strings"
	"text/template"

	sprig "github.com/go-task/slim-sprig/v3"
)

// FuncMap returns Dagu's hermetic template function map.
//
// The map is built from slim-sprig's hermetic text functions, removes
// functions that should not be available in DAG templates, and applies
// Dagu-specific pipeline-friendly overrides.
func FuncMap() template.FuncMap {
	// Start from the hermetic (no env/network/random) slim-sprig set.
	m := sprig.HermeticTxtFuncMap()

	// Defense-in-depth: remove any functions that should never be available in
	// DAG templates. Some of these are not currently present in the hermetic
	// set; keep the blocklist here so future slim-sprig changes cannot expose
	// them accidentally.
	for _, name := range blockedFuncs {
		delete(m, name)
	}

	// Dagu-specific overrides. These preserve pipeline-compatible argument
	// order (pipeline value as last arg) and existing behavior. Each override is
	// intentional; slim-sprig defines overlapping names with different arg order
	// or semantics.
	m["split"] = func(sep, s string) []string {
		return strings.Split(s, sep)
	}
	m["join"] = func(sep string, v any) (string, error) {
		if v == nil {
			return "", nil
		}
		switch elems := v.(type) {
		case []string:
			return strings.Join(elems, sep), nil
		case []any:
			strs := make([]string, len(elems))
			for i, e := range elems {
				strs[i] = fmt.Sprint(e)
			}
			return strings.Join(strs, sep), nil
		default:
			rv := reflect.ValueOf(v)
			if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
				strs := make([]string, rv.Len())
				for i := range strs {
					strs[i] = fmt.Sprint(rv.Index(i).Interface())
				}
				return strings.Join(strs, sep), nil
			}
			return "", fmt.Errorf("join: unsupported type %T", v)
		}
	}
	m["count"] = func(v any) (int, error) {
		if v == nil {
			return 0, nil
		}
		rv := reflect.ValueOf(v)
		switch rv.Kind() { //nolint:exhaustive // unsupported kinds return an error below
		case reflect.Slice, reflect.Map, reflect.Array:
			return rv.Len(), nil
		case reflect.String:
			return rv.Len(), nil
		default:
			return 0, fmt.Errorf("count: unsupported type %T", v)
		}
	}
	m["add"] = func(b, a int) int {
		return a + b
	}
	m["empty"] = func(v any) bool {
		if v == nil {
			return true
		}
		rv := reflect.ValueOf(v)
		switch rv.Kind() { //nolint:exhaustive // non-empty scalar kinds are handled by IsZero below
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
		switch rv.Kind() { //nolint:exhaustive // scalar zero values are handled by IsZero below
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

// blockedFuncs are removed even from the hermetic set as defense-in-depth.
// Some names are not present in slim-sprig v3 today; keep them blocked so
// future or forked slim-sprig versions cannot expose non-hermetic helpers.
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

// BlockedFuncNames returns the names removed from the hermetic slim-sprig
// function map.
func BlockedFuncNames() []string {
	return append([]string(nil), blockedFuncs...)
}
