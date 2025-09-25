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

func TestZombieDetectionConfig(t *testing.T) {
	tests := []struct {
		name             string
		configYAML       string
		expectedInterval time.Duration
		expectWarning    bool
	}{
		{
			name: "DefaultValueWhenNotSpecified",
			configYAML: `
host: "0.0.0.0"
port: 8080
`,
			expectedInterval: 45 * time.Second,
			expectWarning:    false,
		},
		{
			name: "CustomInterval",
			configYAML: `
host: "0.0.0.0"
port: 8080
scheduler:
  zombieDetectionInterval: 2m
`,
			expectedInterval: 2 * time.Minute,
			expectWarning:    false,
		},
		{
			name: "DisabledWithZero",
			configYAML: `
host: "0.0.0.0"
port: 8080
scheduler:
  zombieDetectionInterval: 0s
`,
			expectedInterval: 0, // Should be 0 to disable
			expectWarning:    false,
		},
		{
			name: "InvalidDurationFormat",
			configYAML: `
host: "0.0.0.0"
port: 8080
scheduler:
  zombieDetectionInterval: invalid
`,
			expectedInterval: 0, // Parsing fails, remains 0
			expectWarning:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset viper between tests
			viper.Reset()

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

			// Check for warnings
			if tt.expectWarning {
				assert.NotEmpty(t, cfg.Warnings)
				found := false
				for _, warning := range cfg.Warnings {
					if contains(warning, "zombieDetectionInterval") {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected warning about zombieDetectionInterval")
			}
		})
	}
}

func TestZombieDetectionEnvVar(t *testing.T) {
	// Reset viper between tests
	viper.Reset()

	// Set environment variable with DAGU prefix
	os.Setenv("DAGU_SCHEDULER_ZOMBIE_DETECTION_INTERVAL", "90s")
	defer os.Unsetenv("DAGU_SCHEDULER_ZOMBIE_DETECTION_INTERVAL")

	// Create minimal config
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	configContent := `
host: "0.0.0.0"
port: 8080
`
	err := os.WriteFile(configFile, []byte(configContent), 0600)
	require.NoError(t, err)

	// Load config
	cfg, err := config.Load(config.WithConfigFile(configFile))
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Check that env var overrides default
	assert.Equal(t, 90*time.Second, cfg.Scheduler.ZombieDetectionInterval)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[0:len(s)] != "" &&
		len(substr) > 0 &&
		(s == substr ||
			(len(s) > len(substr) && s[:len(substr)] == substr) ||
			(len(s) > len(substr) && s[len(s)-len(substr):] == substr) ||
			containsMiddle(s, substr))
}

func containsMiddle(s, substr string) bool {
	for i := 1; i < len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
