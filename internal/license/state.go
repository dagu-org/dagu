package license

import (
	"sync"
	"time"
)

const gracePeriod = 14 * 24 * time.Hour

// State implements Checker with thread-safe access to license claims.
type State struct {
	mu     sync.RWMutex
	claims *LicenseClaims
	token  string
}

// Update atomically replaces the license state.
func (s *State) Update(claims *LicenseClaims, token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claims = claims
	s.token = token
}

// Token returns the raw JWT token string.
func (s *State) Token() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.token
}

// Claims returns the current license claims.
func (s *State) Claims() *LicenseClaims {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.claims
}

// IsFeatureEnabled returns true if the feature is available.
// Features remain active during the grace period after expiry.
func (s *State) IsFeatureEnabled(feature string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.claims == nil {
		return false
	}
	if !s.claims.HasFeature(feature) {
		return false
	}

	// If the token has not expired, the feature is enabled.
	if s.claims.ExpiresAt != nil && s.claims.ExpiresAt.After(time.Now()) {
		return true
	}

	// If expired, check grace period.
	return s.isInGracePeriod()
}

// Plan returns the license plan name.
func (s *State) Plan() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.claims == nil {
		return ""
	}
	return s.claims.Plan
}

// IsGracePeriod returns true if the license is expired but within the grace period.
func (s *State) IsGracePeriod() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isInGracePeriod()
}

func (s *State) isInGracePeriod() bool {
	if s.claims == nil || s.claims.ExpiresAt == nil {
		return false
	}
	expiry := s.claims.ExpiresAt.Time
	if time.Now().Before(expiry) {
		return false // not expired yet
	}
	return time.Now().Before(expiry.Add(gracePeriod))
}

// IsCommunity returns true when no license is loaded.
func (s *State) IsCommunity() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.claims == nil
}
