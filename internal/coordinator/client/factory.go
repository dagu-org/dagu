package client

import (
	"context"
	"fmt"
	"time"
)

// Factory creates coordinator clients with a fluent interface
type Factory struct {
	config *Config
}

// NewFactory creates a new client factory with default configuration
func NewFactory() *Factory {
	return &Factory{
		config: DefaultConfig(),
	}
}

// WithConfig sets the entire configuration at once
func (f *Factory) WithConfig(config *Config) *Factory {
	if config != nil {
		f.config = config
	}
	return f
}

// WithHost sets the coordinator host
func (f *Factory) WithHost(host string) *Factory {
	f.config.Host = host
	return f
}

// WithPort sets the coordinator port
func (f *Factory) WithPort(port int) *Factory {
	f.config.Port = port
	return f
}

// WithTLS configures TLS settings with certificate files
func (f *Factory) WithTLS(certFile, keyFile, caFile string) *Factory {
	f.config.Insecure = false
	f.config.CertFile = certFile
	f.config.KeyFile = keyFile
	f.config.CAFile = caFile
	return f
}

// WithInsecure enables insecure connection (no TLS)
func (f *Factory) WithInsecure() *Factory {
	f.config.Insecure = true
	f.config.CertFile = ""
	f.config.KeyFile = ""
	f.config.CAFile = ""
	return f
}

// WithSkipTLSVerify skips server certificate verification
func (f *Factory) WithSkipTLSVerify(skip bool) *Factory {
	f.config.SkipTLSVerify = skip
	return f
}

// WithDialTimeout sets the connection timeout
func (f *Factory) WithDialTimeout(timeout time.Duration) *Factory {
	f.config.DialTimeout = timeout
	return f
}

// WithRequestTimeout sets the per-request timeout
func (f *Factory) WithRequestTimeout(timeout time.Duration) *Factory {
	f.config.RequestTimeout = timeout
	return f
}

// WithRetryConfig sets retry parameters
func (f *Factory) WithRetryConfig(maxRetries int, retryInterval time.Duration) *Factory {
	f.config.MaxRetries = maxRetries
	f.config.RetryInterval = retryInterval
	return f
}

// Build creates a new coordinator client with the configured settings
func (f *Factory) Build(ctx context.Context) (Client, error) {
	// Validate configuration
	if err := f.config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create the client
	client, err := newClient(ctx, f.config)
	if err != nil {
		return nil, err
	}

	return client, nil
}
