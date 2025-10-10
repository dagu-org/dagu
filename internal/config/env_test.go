package config_test

import (
	"os"
	"strings"
	"testing"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/stretchr/testify/require"
)

func TestLoadBaseEnv(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		expected bool
	}{
		{"TEST_VAR_BASE_ENV", false},
		{"DAGU_TEST_BASE_ENV", true},
	}

	for _, tc := range testCases {
		os.Setenv(tc.name, "value")
		t.Cleanup(func() {
			os.Unsetenv(tc.name)
		})
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
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	return envMap
}
