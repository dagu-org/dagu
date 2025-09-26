package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZombieDetectionEnvVarPrecedence(t *testing.T) {
	tests := []struct {
		name             string
		configYAML       string
		envValue         string
		expectedInterval time.Duration
	}{
		{
			name: "EnvVarOverridesDefault",
			configYAML: `
host: "0.0.0.0"
port: 8080
`,
			envValue:         "90s",
			expectedInterval: 90 * time.Second,
		},
		{
			name: "EnvVarOverridesConfigFile",
			configYAML: `
host: "0.0.0.0"
port: 8080
scheduler:
  zombieDetectionInterval: 30s
`,
			envValue:         "120s",
			expectedInterval: 120 * time.Second,
		},
		{
			name: "EnvVarSetToDisable",
			configYAML: `
host: "0.0.0.0"
port: 8080
`,
			envValue:         "0s",
			expectedInterval: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset viper between tests
			viper.Reset()

			// Set environment variable with DAGU prefix
			os.Setenv("DAGU_SCHEDULER_ZOMBIE_DETECTION_INTERVAL", tt.envValue)
			defer os.Unsetenv("DAGU_SCHEDULER_ZOMBIE_DETECTION_INTERVAL")

			// Create temp config file
			tempDir := t.TempDir()
			configFile := filepath.Join(tempDir, "config.yaml")
			err := os.WriteFile(configFile, []byte(tt.configYAML), 0600)
			require.NoError(t, err)

			// Load config
			cfg, err := config.Load(config.WithConfigFile(configFile))
			require.NoError(t, err)
			require.NotNil(t, cfg)

			// Check zombie detection interval
			assert.Equal(t, tt.expectedInterval, cfg.Scheduler.ZombieDetectionInterval)
		})
	}
}
