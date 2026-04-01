// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"os"
	"strings"
)

// BaseEnv represents a filtered set of environment variables to pass to child processes.
type BaseEnv struct {
	variables []string
}

// NewBaseEnv constructs a BaseEnv containing the provided environment variable strings.
// The slice is stored as-is (no copy); callers should pass a defensive copy if isolation is required.
func NewBaseEnv(vars []string) BaseEnv {
	return BaseEnv{variables: vars}
}

// defaultWhitelist defines which env vars to pass to child processes.
// Platform-specific variables are added via init() in env_windows.go and env_unix.go.
var defaultWhitelist = map[string]bool{}

// defaultPrefixes defines prefixes of env vars allowed to propagate.
var defaultPrefixes = []string{
	strings.ToUpper(AppName) + "_", // e.g., "DAGU_"
	"DAG_",                         // Special DAG-related variables
	"LC_",                          // Locale-related variables
	"KUBERNETES_",                  // In-cluster API discovery vars for Kubernetes clients
}

// LoadBaseEnv loads and filters current environment variables.
func LoadBaseEnv() BaseEnv {
	return BaseEnv{variables: filterEnv(os.Environ(), defaultWhitelist, defaultPrefixes)}
}

// AsSlice returns a defensive copy of the filtered environment.
func (b BaseEnv) AsSlice() []string {
	return append([]string{}, b.variables...)
}

func withEnvOverrides(base []string, overrides ...string) []string {
	env := append([]string{}, base...)
	indexByKey := make(map[string]int, len(env))
	for i, entry := range env {
		key, _, found := strings.Cut(entry, "=")
		if found {
			indexByKey[key] = i
		}
	}
	for _, entry := range overrides {
		key, _, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		if idx, ok := indexByKey[key]; ok {
			env[idx] = entry
			continue
		}
		indexByKey[key] = len(env)
		env = append(env, entry)
	}
	return env
}

// case-insensitive matching on Windows).
func filterEnv(envs []string, allow map[string]bool, prefixes []string) []string {
	var filtered []string
	for _, e := range envs {
		key, _, found := strings.Cut(e, "=")
		if !found {
			continue
		}
		// normalizeEnvKey handles platform differences (case-insensitive on Windows)
		if allow[normalizeEnvKey(key)] || hasAllowedPrefix(key, prefixes) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func hasAllowedPrefix(k string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(k, p) {
			return true
		}
	}
	return false
}
