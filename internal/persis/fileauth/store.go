// Package fileauth provides a file-based implementation of auth.AuthStore
// that composes the individual file-based user, API key, and webhook stores.
package fileauth

import (
	"context"
	"fmt"

	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/persis/fileapikey"
	"github.com/dagu-org/dagu/internal/persis/fileuser"
	"github.com/dagu-org/dagu/internal/persis/filewebhook"
	"github.com/dagu-org/dagu/internal/cmn/telemetry"
)

// Config holds configuration for creating a FileAuthStore.
type Config struct {
	UsersDir    string
	APIKeysDir  string
	WebhooksDir string
	CacheLimits config.CacheLimits
	Collector   *telemetry.Collector
}

// Store implements auth.AuthStore by composing file-based sub-stores.
type Store struct {
	users    *fileuser.Store
	apiKeys  *fileapikey.Store
	webhooks *filewebhook.Store
}

var _ auth.AuthStore = (*Store)(nil)

// New creates a file-based AuthStore with caching and telemetry registration.
func New(ctx context.Context, cfg Config) (*Store, error) {
	// User store with cache
	userCache := fileutil.NewCache[*auth.User]("user", cfg.CacheLimits.User.Limit, cfg.CacheLimits.User.TTL)
	userCache.StartEviction(ctx)
	if cfg.Collector != nil {
		cfg.Collector.RegisterCache(userCache)
	}
	userStore, err := fileuser.New(cfg.UsersDir, fileuser.WithFileCache(userCache))
	if err != nil {
		return nil, fmt.Errorf("fileauth: failed to create user store: %w", err)
	}

	// API key store with cache
	apiKeyCache := fileutil.NewCache[*auth.APIKey]("api_key", cfg.CacheLimits.APIKey.Limit, cfg.CacheLimits.APIKey.TTL)
	apiKeyCache.StartEviction(ctx)
	if cfg.Collector != nil {
		cfg.Collector.RegisterCache(apiKeyCache)
	}
	apiKeyStore, err := fileapikey.New(cfg.APIKeysDir, fileapikey.WithFileCache(apiKeyCache))
	if err != nil {
		return nil, fmt.Errorf("fileauth: failed to create API key store: %w", err)
	}

	// Webhook store with cache
	webhookCache := fileutil.NewCache[*auth.Webhook]("webhook", cfg.CacheLimits.Webhook.Limit, cfg.CacheLimits.Webhook.TTL)
	webhookCache.StartEviction(ctx)
	if cfg.Collector != nil {
		cfg.Collector.RegisterCache(webhookCache)
	}
	webhookStore, err := filewebhook.New(cfg.WebhooksDir, filewebhook.WithFileCache(webhookCache))
	if err != nil {
		return nil, fmt.Errorf("fileauth: failed to create webhook store: %w", err)
	}

	return &Store{
		users:    userStore,
		apiKeys:  apiKeyStore,
		webhooks: webhookStore,
	}, nil
}

// Users returns the user store.
func (s *Store) Users() auth.UserStore { return s.users }

// APIKeys returns the API key store.
func (s *Store) APIKeys() auth.APIKeyStore { return s.apiKeys }

// Webhooks returns the webhook store.
func (s *Store) Webhooks() auth.WebhookStore { return s.webhooks }
