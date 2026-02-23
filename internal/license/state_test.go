package license

import (
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestState_ZeroValue(t *testing.T) {
	t.Parallel()

	t.Run("Claims returns nil", func(t *testing.T) {
		t.Parallel()
		var s State
		assert.Nil(t, s.Claims())
	})

	t.Run("Token returns empty string", func(t *testing.T) {
		t.Parallel()
		var s State
		assert.Equal(t, "", s.Token())
	})

	t.Run("IsCommunity returns true", func(t *testing.T) {
		t.Parallel()
		var s State
		assert.True(t, s.IsCommunity())
	})

	t.Run("Plan returns empty string", func(t *testing.T) {
		t.Parallel()
		var s State
		assert.Equal(t, "", s.Plan())
	})
}

func TestState_Update(t *testing.T) {
	t.Parallel()

	t.Run("set claims then read back all fields", func(t *testing.T) {
		t.Parallel()
		var s State
		claims := validClaims()
		token := "test-token-value"

		s.Update(claims, token)

		gotClaims := s.Claims()
		require.NotNil(t, gotClaims)
		assert.Equal(t, claims.Plan, gotClaims.Plan)
		assert.Equal(t, claims.Features, gotClaims.Features)
		assert.Equal(t, claims.ActivationID, gotClaims.ActivationID)
		assert.Equal(t, token, s.Token())
		assert.False(t, s.IsCommunity())
		assert.Equal(t, "pro", s.Plan())
	})

	t.Run("set claims then clear to nil", func(t *testing.T) {
		t.Parallel()
		var s State
		s.Update(validClaims(), "some-token")

		// Verify state was set.
		require.NotNil(t, s.Claims())
		require.Equal(t, "some-token", s.Token())

		// Now clear.
		s.Update(nil, "")

		assert.Nil(t, s.Claims())
		assert.Equal(t, "", s.Token())
		assert.True(t, s.IsCommunity())
		assert.Equal(t, "", s.Plan())
	})
}

func TestState_IsFeatureEnabled(t *testing.T) {
	t.Parallel()

	t.Run("valid token with feature present returns true", func(t *testing.T) {
		t.Parallel()
		var s State
		s.Update(validClaims(), "tok")
		assert.True(t, s.IsFeatureEnabled(FeatureAudit))
		assert.True(t, s.IsFeatureEnabled(FeatureRBAC))
		assert.True(t, s.IsFeatureEnabled(FeatureSSO))
	})

	t.Run("nil claims returns false", func(t *testing.T) {
		t.Parallel()
		var s State
		assert.False(t, s.IsFeatureEnabled(FeatureAudit))
	})

	t.Run("feature absent returns false", func(t *testing.T) {
		t.Parallel()
		var s State
		claims := &LicenseClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			},
			Plan:     "pro",
			Features: []string{FeatureRBAC},
		}
		s.Update(claims, "tok")
		assert.False(t, s.IsFeatureEnabled(FeatureAudit))
		assert.False(t, s.IsFeatureEnabled(FeatureSSO))
		assert.True(t, s.IsFeatureEnabled(FeatureRBAC))
	})

	t.Run("expired in grace period returns true", func(t *testing.T) {
		t.Parallel()
		var s State
		s.Update(expiredInGraceClaims(), "tok")
		assert.True(t, s.IsFeatureEnabled(FeatureAudit))
		assert.True(t, s.IsFeatureEnabled(FeatureRBAC))
		assert.True(t, s.IsFeatureEnabled(FeatureSSO))
	})

	t.Run("expired past grace period returns false", func(t *testing.T) {
		t.Parallel()
		var s State
		s.Update(expiredPastGraceClaims(), "tok")
		assert.False(t, s.IsFeatureEnabled(FeatureAudit))
		assert.False(t, s.IsFeatureEnabled(FeatureRBAC))
		assert.False(t, s.IsFeatureEnabled(FeatureSSO))
	})

	t.Run("nil ExpiresAt returns false", func(t *testing.T) {
		t.Parallel()
		var s State
		// ExpiresAt is nil: the first condition in IsFeatureEnabled fails (nil != nil is false but
		// the nil check means the branch is not taken), then isInGracePeriod also returns false for
		// nil ExpiresAt.
		claims := &LicenseClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: nil,
			},
			Plan:     "pro",
			Features: []string{FeatureAudit, FeatureRBAC, FeatureSSO},
		}
		s.Update(claims, "tok")
		assert.False(t, s.IsFeatureEnabled(FeatureAudit))
		assert.False(t, s.IsFeatureEnabled(FeatureRBAC))
		assert.False(t, s.IsFeatureEnabled(FeatureSSO))
	})
}

func TestState_IsGracePeriod(t *testing.T) {
	t.Parallel()

	t.Run("not expired returns false", func(t *testing.T) {
		t.Parallel()
		var s State
		s.Update(validClaims(), "tok")
		assert.False(t, s.IsGracePeriod())
	})

	t.Run("just expired 1 day ago returns true", func(t *testing.T) {
		t.Parallel()
		var s State
		s.Update(expiredInGraceClaims(), "tok")
		assert.True(t, s.IsGracePeriod())
	})

	t.Run("boundary 14 days minus 1 second returns true", func(t *testing.T) {
		t.Parallel()
		var s State
		// Expired exactly (14d - 1s) ago — still within grace window.
		expiry := time.Now().Add(-(gracePeriod - time.Second))
		claims := &LicenseClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(expiry),
			},
			Plan:     "pro",
			Features: []string{FeatureAudit},
		}
		s.Update(claims, "tok")
		assert.True(t, s.IsGracePeriod())
	})

	t.Run("boundary 14 days plus 1 second returns false", func(t *testing.T) {
		t.Parallel()
		var s State
		// Expired (14d + 1s) ago — just past the grace window.
		expiry := time.Now().Add(-(gracePeriod + time.Second))
		claims := &LicenseClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(expiry),
			},
			Plan:     "pro",
			Features: []string{FeatureAudit},
		}
		s.Update(claims, "tok")
		assert.False(t, s.IsGracePeriod())
	})

	t.Run("nil claims returns false", func(t *testing.T) {
		t.Parallel()
		var s State
		assert.False(t, s.IsGracePeriod())
	})

	t.Run("nil ExpiresAt returns false", func(t *testing.T) {
		t.Parallel()
		var s State
		claims := &LicenseClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: nil,
			},
			Plan:     "pro",
			Features: []string{FeatureAudit},
		}
		s.Update(claims, "tok")
		assert.False(t, s.IsGracePeriod())
	})
}

func TestState_Concurrency(t *testing.T) {
	t.Parallel()

	// This test validates that concurrent access to State does not race.
	// Run with -race to detect any data races.
	var s State

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(i int) {
			defer wg.Done()
			switch i % 7 {
			case 0:
				s.Update(validClaims(), "tok-valid")
			case 1:
				s.Update(expiredInGraceClaims(), "tok-grace")
			case 2:
				s.Update(nil, "")
			case 3:
				_ = s.Claims()
			case 4:
				_ = s.Token()
			case 5:
				_ = s.IsFeatureEnabled(FeatureAudit)
				_ = s.IsFeatureEnabled(FeatureRBAC)
				_ = s.IsFeatureEnabled(FeatureSSO)
			case 6:
				_ = s.Plan()
				_ = s.IsGracePeriod()
				_ = s.IsCommunity()
			}
		}(i)
	}

	wg.Wait()
	// No assertion needed — the test passes if no race is detected.
}

// TestState_Plan verifies Plan() independently of the Update round-trip test.
func TestState_Plan(t *testing.T) {
	t.Parallel()

	t.Run("returns plan from claims", func(t *testing.T) {
		t.Parallel()
		var s State
		s.Update(validClaims(), "tok")
		assert.Equal(t, "pro", s.Plan())
	})

	t.Run("returns empty string when nil claims", func(t *testing.T) {
		t.Parallel()
		var s State
		assert.Equal(t, "", s.Plan())
	})
}

// TestState_IsCommunity verifies community detection beyond the zero-value case.
func TestState_IsCommunity(t *testing.T) {
	t.Parallel()

	t.Run("returns false when claims are loaded", func(t *testing.T) {
		t.Parallel()
		var s State
		s.Update(validClaims(), "tok")
		assert.False(t, s.IsCommunity())
	})

	t.Run("returns true after claims cleared to nil", func(t *testing.T) {
		t.Parallel()
		var s State
		s.Update(validClaims(), "tok")
		s.Update(nil, "")
		assert.True(t, s.IsCommunity())
	})
}

// Compile-time check: *State satisfies the Checker interface.
var _ Checker = (*State)(nil)
