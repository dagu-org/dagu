// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package buildenv

import (
	"os"
	"slices"
	"strings"
)

// PresolvedEnvPrefix is the env var prefix used to transport pre-resolved
// DAG/base-config env values from a parent process to a subprocess.
const PresolvedEnvPrefix = "_DAGU_PRESOLVED_BUILD_ENV_"

// Encode converts resolved env entries into transport env vars.
// Duplicate keys are collapsed so the last value wins.
func Encode(env []string) []string {
	entries := ToMap(env)
	if len(entries) == 0 {
		return nil
	}

	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	extra := make([]string, 0, len(keys))
	for _, key := range keys {
		extra = append(extra, PresolvedEnvPrefix+key+"="+entries[key])
	}
	return extra
}

// Load returns the pre-resolved build env currently present in the process
// environment.
func Load() map[string]string {
	entries := make(map[string]string)
	for _, env := range os.Environ() {
		key, value, ok := strings.Cut(env, "=")
		if !ok {
			continue
		}
		name, ok := strings.CutPrefix(key, PresolvedEnvPrefix)
		if !ok || name == "" {
			continue
		}
		entries[name] = value
	}
	if len(entries) == 0 {
		return nil
	}
	return entries
}

// ToMap converts env entries into a map. Duplicate keys are collapsed so the
// last value wins.
func ToMap(env []string) map[string]string {
	if len(env) == 0 {
		return nil
	}

	entries := make(map[string]string)
	for _, item := range env {
		key, value, ok := strings.Cut(item, "=")
		if !ok || key == "" {
			continue
		}
		entries[key] = value
	}
	if len(entries) == 0 {
		return nil
	}
	return entries
}
