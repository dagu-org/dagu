package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	t.Parallel()
	t.Run("ValidConfig", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			Server: Server{
				Port: 8080,
			},
			UI: UI{
				MaxDashboardPageLimit: 100,
			},
		}
		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("InvalidPort_Negative", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			Server: Server{
				Port: -1,
			},
			UI: UI{
				MaxDashboardPageLimit: 100,
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port number")
	})

	t.Run("InvalidPort_TooLarge", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			Server: Server{
				Port: 99999,
			},
			UI: UI{
				MaxDashboardPageLimit: 100,
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port number")
	})

	t.Run("InvalidPort_MaxValue", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			Server: Server{
				Port: 65536,
			},
			UI: UI{
				MaxDashboardPageLimit: 100,
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port number")
	})

	t.Run("ValidPort_MinValue", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			Server: Server{
				Port: 0,
			},
			UI: UI{
				MaxDashboardPageLimit: 100,
			},
		}
		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("ValidPort_MaxValue", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			Server: Server{
				Port: 65535,
			},
			UI: UI{
				MaxDashboardPageLimit: 100,
			},
		}
		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("IncompleteTLS_MissingCert", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			Server: Server{
				Port: 8080,
				TLS: &TLSConfig{
					KeyFile: "/path/to/key.pem",
				},
			},
			UI: UI{
				MaxDashboardPageLimit: 100,
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "TLS configuration incomplete")
	})

	t.Run("IncompleteTLS_MissingKey", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			Server: Server{
				Port: 8080,
				TLS: &TLSConfig{
					CertFile: "/path/to/cert.pem",
				},
			},
			UI: UI{
				MaxDashboardPageLimit: 100,
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "TLS configuration incomplete")
	})

	t.Run("CompleteTLS", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			Server: Server{
				Port: 8080,
				TLS: &TLSConfig{
					CertFile: "/path/to/cert.pem",
					KeyFile:  "/path/to/key.pem",
				},
			},
			UI: UI{
				MaxDashboardPageLimit: 100,
			},
		}
		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("InvalidMaxDashboardPageLimit_Zero", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			Server: Server{
				Port: 8080,
			},
			UI: UI{
				MaxDashboardPageLimit: 0,
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid max dashboard page limit")
	})

	t.Run("InvalidMaxDashboardPageLimit_Negative", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			Server: Server{
				Port: 8080,
			},
			UI: UI{
				MaxDashboardPageLimit: -1,
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid max dashboard page limit")
	})

	t.Run("ValidMaxDashboardPageLimit_MinValue", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			Server: Server{
				Port: 8080,
			},
			UI: UI{
				MaxDashboardPageLimit: 1,
			},
		}
		err := cfg.Validate()
		require.NoError(t, err)
	})
}
