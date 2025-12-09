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

// ConfigFileUsed retrieves the path to the configuration file used
func ConfigFileUsed(ctx context.Context) string {
	if cfg := GetConfig(ctx); cfg != nil {
		return cfg.Paths.ConfigFileUsed
	}
	return ""
}

// GetBaseEnv returns the BaseEnv from the configuration stored in ctx.
// If no configuration is present in the context, it returns nil.
func GetBaseEnv(ctx context.Context) *BaseEnv {
	if cfg := GetConfig(ctx); cfg != nil {
		return &cfg.Core.BaseEnv
	}
	return nil
}