package config

import (
	"context"
)

type configKey struct{}

const warningNoConfigInContext = "Configuration not found in context, returning empty config"

// WithConfig creates a new context with the provided configuration.
func WithConfig(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, configKey{}, cfg)
}

// GetConfig retrieves the configuration from the context.
func GetConfig(ctx context.Context) *Config {
	if cfg, ok := ctx.Value(configKey{}).(*Config); ok {
		return cfg
	}
	warnings := []string{warningNoConfigInContext}
	return &Config{Warnings: warnings}
}
