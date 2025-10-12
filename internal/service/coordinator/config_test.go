package coordinator_test

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/service/coordinator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := coordinator.DefaultConfig()

	assert.True(t, config.Insecure)
	assert.Equal(t, 10*time.Second, config.DialTimeout)
	assert.Equal(t, 5*time.Minute, config.RequestTimeout)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, time.Second, config.RetryInterval)
	assert.Empty(t, config.CertFile)
	assert.Empty(t, config.KeyFile)
	assert.Empty(t, config.CAFile)
	assert.False(t, config.SkipTLSVerify)
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *coordinator.Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "DefaultConfig",
			config:  coordinator.DefaultConfig(),
			wantErr: false,
		},
		{
			name: "InsecureMode",
			config: &coordinator.Config{
				Insecure:       true,
				DialTimeout:    10 * time.Second,
				RequestTimeout: 5 * time.Minute,
				MaxRetries:     3,
				RetryInterval:  time.Second,
			},
			wantErr: false,
		},
		{
			name: "TLSWithCerts",
			config: &coordinator.Config{
				Insecure:       false,
				CertFile:       "/path/to/cert.pem",
				KeyFile:        "/path/to/key.pem",
				CAFile:         "/path/to/ca.pem",
				DialTimeout:    10 * time.Second,
				RequestTimeout: 5 * time.Minute,
				MaxRetries:     3,
				RetryInterval:  time.Second,
			},
			wantErr: false,
		},
		{
			name: "TLSWithoutCerts",
			config: &coordinator.Config{
				Insecure:       false,
				DialTimeout:    10 * time.Second,
				RequestTimeout: 5 * time.Minute,
				MaxRetries:     3,
				RetryInterval:  time.Second,
			},
			wantErr: true,
			errMsg:  "TLS enabled but no certificates provided",
		},
		{
			name: "ZeroDialTimeout",
			config: &coordinator.Config{
				Insecure:       true,
				DialTimeout:    0,
				RequestTimeout: 5 * time.Minute,
				MaxRetries:     3,
				RetryInterval:  time.Second,
			},
			wantErr: false,
		},
		{
			name: "ZeroRequestTimeout",
			config: &coordinator.Config{
				Insecure:       true,
				DialTimeout:    10 * time.Second,
				RequestTimeout: 0,
				MaxRetries:     3,
				RetryInterval:  time.Second,
			},
			wantErr: false,
		},
		{
			name: "NegativeMaxRetries",
			config: &coordinator.Config{
				Insecure:       true,
				DialTimeout:    10 * time.Second,
				RequestTimeout: 5 * time.Minute,
				MaxRetries:     -1,
				RetryInterval:  time.Second,
			},
			wantErr: false,
		},
		{
			name: "ZeroRetryInterval",
			config: &coordinator.Config{
				Insecure:       true,
				DialTimeout:    10 * time.Second,
				RequestTimeout: 5 * time.Minute,
				MaxRetries:     3,
				RetryInterval:  0,
			},
			wantErr: false,
		},
		{
			name: "TLSWithSkipVerify",
			config: &coordinator.Config{
				Insecure:       false,
				SkipTLSVerify:  true,
				CertFile:       "/path/to/cert.pem",
				KeyFile:        "/path/to/key.pem",
				DialTimeout:    10 * time.Second,
				RequestTimeout: 5 * time.Minute,
				MaxRetries:     3,
				RetryInterval:  time.Second,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)

				// Check defaults are applied
				if tt.config.DialTimeout == 0 {
					assert.Equal(t, 10*time.Second, tt.config.DialTimeout)
				}
				if tt.config.RequestTimeout == 0 {
					assert.Equal(t, 5*time.Minute, tt.config.RequestTimeout)
				}
				if tt.config.MaxRetries < 0 {
					assert.Equal(t, 0, tt.config.MaxRetries)
				}
				if tt.config.RetryInterval == 0 {
					assert.Equal(t, time.Second, tt.config.RetryInterval)
				}
			}
		})
	}
}

func TestConfigValidateDefaults(t *testing.T) {
	// Test that Validate sets proper defaults
	config := &coordinator.Config{
		Insecure:       true,
		DialTimeout:    0,
		RequestTimeout: 0,
		MaxRetries:     -5,
		RetryInterval:  0,
	}

	err := config.Validate()
	require.NoError(t, err)

	assert.Equal(t, 10*time.Second, config.DialTimeout)
	assert.Equal(t, 5*time.Minute, config.RequestTimeout)
	assert.Equal(t, 0, config.MaxRetries)
	assert.Equal(t, time.Second, config.RetryInterval)
}
