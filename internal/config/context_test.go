package config_test

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/config"
)

func TestWithConfigAndGetConfig(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{
		Server: config.Server{
			Host: "localhost",
			Port: 8080,
		},
	}

	// Store config in context
	ctx = config.WithConfig(ctx, cfg)

	// Retrieve config from context
	retrieved := config.GetConfig(ctx)

	if retrieved != cfg {
		t.Errorf("expected config to be %v, got %v", cfg, retrieved)
	}
	if retrieved.Server.Host != "localhost" {
		t.Errorf("expected Host to be 'localhost', got %s", retrieved.Server.Host)
	}
	if retrieved.Server.Port != 8080 {
		t.Errorf("expected Port to be 8080, got %d", retrieved.Server.Port)
	}
}

func TestGetConfig_NoConfigInContext(t *testing.T) {
	ctx := context.Background()

	// Get config from context without setting it
	cfg := config.GetConfig(ctx)

	if cfg == nil {
		t.Error("expected non-nil config, got nil")
	}
	// Should return empty config with zero values
	if cfg.Server.Host != "" || cfg.Server.Port != 0 {
		t.Errorf("expected empty config, got Server: %+v", cfg.Server)
	}
}
