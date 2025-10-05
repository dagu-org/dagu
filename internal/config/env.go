package config

import (
	"os"
	"strings"

	"github.com/dagu-org/dagu/internal/build"
)

// BaseEnv represents a filtered set of environment variables to pass to child processes.
type BaseEnv struct {
	variables []string
}

// DefaultWhitelist defines which env vars to pass to child processes.
var DefaultWhitelist = map[string]bool{
	"PATH": true,
	"HOME": true,
	"LANG": true,
	"TZ":   true,
}

// DefaultPrefixes defines prefixes of env vars allowed to propagate.
var DefaultPrefixes = []string{strings.ToUpper(build.AppName) + "_"}

// LoadBaseEnv loads and filters current environment variables.
func LoadBaseEnv() BaseEnv {
	return BaseEnv{variables: filterEnv(os.Environ(), DefaultWhitelist, DefaultPrefixes)}
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
