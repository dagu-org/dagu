// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package config_test

import (
	"strings"
	"testing"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/stretchr/testify/require"
)

func TestLoadBaseEnv(t *testing.T) {
	testCases := []struct {
		name     string
		expected bool
	}{
		{"TEST_VAR_BASE_ENV", false},
		{"DAGU_TEST_BASE_ENV", true},

		// Docker daemon connection vars must pass through.
		{"DOCKER_HOST", true},
		{"DOCKER_TLS_VERIFY", true},
		{"DOCKER_CERT_PATH", true},
		{"DOCKER_API_VERSION", true},

		// Docker credentials must NOT leak through the global env.
		{"DOCKER_AUTH_CONFIG", false},
	}

	for _, tc := range testCases {
		t.Setenv(tc.name, "value")
	}

	baseEnv := config.LoadBaseEnv()
	envSlice := baseEnv.AsSlice()
	envMap := parseEnvSlice(envSlice)

	for _, tc := range testCases {
		_, found := envMap[tc.name]
		require.Equal(t, tc.expected, found, "expected %s: %v", tc.name, tc.expected)
	}
}

func TestBaseEnv_AsSlice(t *testing.T) {
	t.Parallel()

	baseEnv := config.NewBaseEnv([]string{"A=1", "B=2"})
	require.Equal(t, []string{"A=1", "B=2"}, baseEnv.AsSlice())
}

func parseEnvSlice(envSlice []string) map[string]string {
	envMap := make(map[string]string)
	for _, kv := range envSlice {
		key, value, found := strings.Cut(kv, "=")
		if found {
			envMap[key] = value
		}
	}
	return envMap
}
