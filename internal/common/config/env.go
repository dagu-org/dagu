package config

import (
	"os"
	"strings"
)

// BaseEnv represents a filtered set of environment variables to pass to child processes.
type BaseEnv struct {
	variables []string
}

// NewBaseEnv creates a new BaseEnv with the provided variables.
func NewBaseEnv(vars []string) BaseEnv {
	return BaseEnv{variables: vars}
}

// defaultWhitelist defines which env vars to pass to child processes.
var defaultWhitelist = map[string]bool{
	"PATH":            true,
	"HOME":            true,
	"LANG":            true,
	"TZ":              true,
	"SHELL":           true,
	"LD_LIBRARY_PATH": true,
}

// defaultPrefixes defines prefixes of env vars allowed to propagate.
var defaultPrefixes = []string{
	strings.ToUpper(AppName) + "_", // e.g., "DAGU_"
	"DAG_",                         // Special DAG-related variables
	"LC_",                          // Locale-related variables
}

// LoadBaseEnv loads and filters current environment variables.
func LoadBaseEnv() BaseEnv {
	return BaseEnv{variables: filterEnv(os.Environ(), defaultWhitelist, defaultPrefixes)}
}

// AsSlice returns a defensive copy of the filtered environment.
func (b BaseEnv) AsSlice() []string {
	return append([]string{}, b.variables...)
}

// filterEnv filters the provided environment variables.
func filterEnv(envs []string, allow map[string]bool, prefixes []string) []string {
	var filtered []string
	for _, e := range envs {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		if allow[key] || hasAllowedPrefix(key, prefixes) {
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
