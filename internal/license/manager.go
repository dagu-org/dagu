package license

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

const heartbeatInterval = 24 * time.Hour

// ManagerConfig holds the configuration for the license manager.
type ManagerConfig struct {
	LicenseDir string
	ConfigKey  string
	CloudURL   string
}

// ActivationResult is returned after a successful activation.
type ActivationResult struct {
	Plan     string
	Features []string
	Expiry   time.Time
}

// Manager orchestrates license discovery, activation, verification, and heartbeat.
type Manager struct {
	cfg    ManagerConfig
	state  *State
	store  ActivationStore
	client *CloudClient
	pubKey ed25519.PublicKey
	logger *slog.Logger

	cancelMu sync.Mutex
	cancel   context.CancelFunc
	wg       sync.WaitGroup

	heartbeatOnce sync.Once
}

// NewManager creates a new license manager.
func NewManager(cfg ManagerConfig, pubKey ed25519.PublicKey, store ActivationStore, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		cfg:    cfg,
		state:  &State{},
		store:  store,
		client: NewCloudClient(cfg.CloudURL),
		pubKey: pubKey,
		logger: logger,
	}
}

// Checker returns the Checker interface backed by the manager's state.
func (m *Manager) Checker() Checker {
	return m.state
}

// Start performs discovery, optional activation, JWT verification, and starts the heartbeat loop.
func (m *Manager) Start(ctx context.Context) error {
	result, err := Discover(m.cfg.LicenseDir, m.cfg.ConfigKey, m.store)
	if err != nil {
		m.logger.Warn("License discovery failed", slog.String("error", err.Error()))
		return nil // graceful degradation
	}

	if result.Source == SourceNone {
		m.logger.Debug("No license configured, running in community mode")
		return nil
	}

	// If we have a key but no token, activate first
	if result.LicenseKey != "" && result.Token == "" {
		activationResult, activateErr := m.activate(ctx, result.LicenseKey)
		if activateErr != nil {
			m.logger.Warn("License activation failed, running in community mode",
				slog.String("error", activateErr.Error()))
			return nil // graceful degradation
		}
		result.Token = activationResult.Token
		result.Activation = activationResult
	}

	// Verify the token
	claims, verifyErr := VerifyToken(m.pubKey, result.Token)
	if verifyErr != nil {
		// Try lenient verification for grace period
		claims, verifyErr = VerifyTokenLenient(m.pubKey, result.Token)
		if verifyErr != nil {
			m.logger.Warn("License token verification failed",
				slog.String("error", verifyErr.Error()))
			return nil // graceful degradation
		}
		m.logger.Warn("License token is expired, operating in grace period")
	}

	m.state.Update(claims, result.Token)

	m.logger.Info("License loaded",
		slog.String("plan", claims.Plan),
		slog.Any("features", claims.Features),
		slog.String("source", fmt.Sprintf("%d", result.Source)),
	)

	// Start heartbeat loop if the source requires it
	if result.Source.NeedsHeartbeat() && result.Activation != nil {
		m.startHeartbeat(result.Activation)
	}

	return nil
}

// Stop cancels the heartbeat goroutine and waits for completion.
func (m *Manager) Stop() {
	m.cancelMu.Lock()
	cancel := m.cancel
	m.cancelMu.Unlock()
	if cancel != nil {
		cancel()
	}
	m.wg.Wait()
}

// ActivateWithKey performs activation with the given key and updates internal state.
// This is used by the API handler for frontend-initiated activation.
func (m *Manager) ActivateWithKey(ctx context.Context, key string) (*ActivationResult, error) {
	ad, err := m.activate(ctx, key)
	if err != nil {
		return nil, err
	}

	claims, verifyErr := VerifyToken(m.pubKey, ad.Token)
	if verifyErr != nil {
		return nil, fmt.Errorf("activated token verification failed: %w", verifyErr)
	}

	m.state.Update(claims, ad.Token)

	// Start heartbeat if not already running
	m.startHeartbeat(ad)

	result := &ActivationResult{
		Plan:     claims.Plan,
		Features: claims.Features,
	}
	if claims.ExpiresAt != nil {
		result.Expiry = claims.ExpiresAt.Time
	}
	return result, nil
}

func (m *Manager) activate(ctx context.Context, key string) (*ActivationData, error) {
	serverID, err := GetOrCreateServerID(m.cfg.LicenseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get server ID: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		m.logger.Warn("Failed to get hostname", slog.String("error", err.Error()))
		hostname = "unknown"
	}

	resp, err := m.client.Activate(ctx, ActivateRequest{
		Key:         key,
		ServerID:    serverID,
		MachineName: hostname,
	})
	if err != nil {
		return nil, fmt.Errorf("activation request failed: %w", err)
	}

	ad := &ActivationData{
		Token:           resp.Token,
		HeartbeatSecret: resp.HeartbeatSecret,
		LicenseKey:      key,
		ServerID:        serverID,
	}

	if m.store != nil {
		if err := m.store.Save(ad); err != nil {
			m.logger.Warn("Failed to persist activation data", slog.String("error", err.Error()))
		}
	}

	return ad, nil
}

func (m *Manager) startHeartbeat(ad *ActivationData) {
	m.heartbeatOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		m.cancelMu.Lock()
		m.cancel = cancel
		m.cancelMu.Unlock()
		m.wg.Add(1)
		go m.heartbeatLoop(ctx, ad)
	})
}

func (m *Manager) heartbeatLoop(ctx context.Context, ad *ActivationData) {
	defer m.wg.Done()

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.doHeartbeat(ctx, ad)
		}
	}
}

func (m *Manager) doHeartbeat(ctx context.Context, ad *ActivationData) {
	claims := m.state.Claims()
	if claims == nil {
		return
	}

	resp, err := m.client.Heartbeat(ctx, HeartbeatRequest{
		LicenseID:       claims.ActivationID,
		ServerID:        ad.ServerID,
		HeartbeatSecret: ad.HeartbeatSecret,
	})
	if err != nil {
		var cloudErr *CloudError
		if errors.As(err, &cloudErr) {
			switch cloudErr.StatusCode {
			case 410: // Gone - license revoked
				m.logger.Error("License has been revoked")
				m.state.Update(nil, "")
				if m.store != nil {
					_ = m.store.Remove()
				}
				return
			case 401: // Unauthorized - re-activation needed
				m.logger.Error("License heartbeat unauthorized, re-activation may be needed",
					slog.String("error", cloudErr.Message))
				return
			}
		}
		// Network error or other transient failure - continue with cached JWT
		m.logger.Warn("License heartbeat failed, continuing with cached token",
			slog.String("error", err.Error()))
		return
	}

	// Verify the refreshed token
	newClaims, verifyErr := VerifyToken(m.pubKey, resp.Token)
	if verifyErr != nil {
		m.logger.Warn("Refreshed token verification failed",
			slog.String("error", verifyErr.Error()))
		return
	}

	m.state.Update(newClaims, resp.Token)

	// Persist the refreshed token
	if m.store != nil {
		ad.Token = resp.Token
		if err := m.store.Save(ad); err != nil {
			m.logger.Warn("Failed to persist refreshed token",
				slog.String("error", err.Error()))
		}
	}

	m.logger.Debug("License heartbeat successful",
		slog.String("plan", newClaims.Plan))
}
