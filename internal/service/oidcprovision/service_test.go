// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package oidcprovision

import (
	"context"
	"fmt"
	"testing"

	"github.com/dagu-org/dagu/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockUserStore implements auth.UserStore for testing.
type mockUserStore struct {
	users          map[string]*auth.User
	byUsername     map[string]*auth.User
	byOIDCIdentity map[string]*auth.User
}

func newMockUserStore() *mockUserStore {
	return &mockUserStore{
		users:          make(map[string]*auth.User),
		byUsername:     make(map[string]*auth.User),
		byOIDCIdentity: make(map[string]*auth.User),
	}
}

func (m *mockUserStore) Create(_ context.Context, user *auth.User) error {
	if _, exists := m.users[user.ID]; exists {
		return auth.ErrUserAlreadyExists
	}
	if _, exists := m.byUsername[user.Username]; exists {
		return auth.ErrUserAlreadyExists
	}
	m.users[user.ID] = user
	m.byUsername[user.Username] = user
	if user.OIDCIssuer != "" && user.OIDCSubject != "" {
		key := user.OIDCIssuer + ":" + user.OIDCSubject
		m.byOIDCIdentity[key] = user
	}
	return nil
}

func (m *mockUserStore) GetByID(_ context.Context, id string) (*auth.User, error) {
	user, exists := m.users[id]
	if !exists {
		return nil, auth.ErrUserNotFound
	}
	return user, nil
}

func (m *mockUserStore) GetByUsername(_ context.Context, username string) (*auth.User, error) {
	user, exists := m.byUsername[username]
	if !exists {
		return nil, auth.ErrUserNotFound
	}
	return user, nil
}

func (m *mockUserStore) GetByOIDCIdentity(_ context.Context, issuer, subject string) (*auth.User, error) {
	key := issuer + ":" + subject
	user, exists := m.byOIDCIdentity[key]
	if !exists {
		return nil, auth.ErrOIDCIdentityNotFound
	}
	return user, nil
}

func (m *mockUserStore) List(_ context.Context) ([]*auth.User, error) {
	users := make([]*auth.User, 0, len(m.users))
	for _, u := range m.users {
		users = append(users, u)
	}
	return users, nil
}

func (m *mockUserStore) Update(_ context.Context, user *auth.User) error {
	if _, exists := m.users[user.ID]; !exists {
		return auth.ErrUserNotFound
	}
	m.users[user.ID] = user
	m.byUsername[user.Username] = user
	return nil
}

func (m *mockUserStore) Delete(_ context.Context, id string) error {
	user, exists := m.users[id]
	if !exists {
		return auth.ErrUserNotFound
	}
	delete(m.users, id)
	delete(m.byUsername, user.Username)
	if user.OIDCIssuer != "" && user.OIDCSubject != "" {
		key := user.OIDCIssuer + ":" + user.OIDCSubject
		delete(m.byOIDCIdentity, key)
	}
	return nil
}

func (m *mockUserStore) Count(_ context.Context) (int64, error) {
	return int64(len(m.users)), nil
}

func TestProcessLogin_NewUser(t *testing.T) {
	store := newMockUserStore()
	svc := New(store, Config{
		Issuer:      "https://issuer.example.com",
		AutoSignup:  true,
		DefaultRole: auth.RoleViewer,
	})

	claims := OIDCClaims{
		Subject:           "sub123",
		Email:             "user@example.com",
		PreferredUsername: "testuser",
	}

	user, isNew, err := svc.ProcessLogin(context.Background(), claims)
	require.NoError(t, err)
	assert.True(t, isNew)
	assert.NotNil(t, user)
	assert.Equal(t, "testuser", user.Username)
	assert.Equal(t, auth.RoleViewer, user.Role)
	assert.Equal(t, "oidc", user.AuthProvider)
	assert.Equal(t, "https://issuer.example.com", user.OIDCIssuer)
	assert.Equal(t, "sub123", user.OIDCSubject)
}

func TestProcessLogin_ExistingUser(t *testing.T) {
	store := newMockUserStore()
	existingUser := &auth.User{
		ID:           "existing-id",
		Username:     "existinguser",
		Role:         auth.RoleOperator,
		AuthProvider: "oidc",
		OIDCIssuer:   "https://issuer.example.com",
		OIDCSubject:  "sub123",
	}
	_ = store.Create(context.Background(), existingUser)

	svc := New(store, Config{
		Issuer:      "https://issuer.example.com",
		AutoSignup:  true,
		DefaultRole: auth.RoleViewer,
	})

	claims := OIDCClaims{
		Subject:           "sub123",
		Email:             "user@example.com",
		PreferredUsername: "testuser",
	}

	user, isNew, err := svc.ProcessLogin(context.Background(), claims)
	require.NoError(t, err)
	assert.False(t, isNew)
	assert.Equal(t, "existing-id", user.ID)
	assert.Equal(t, "existinguser", user.Username)
	// Role should not change on subsequent logins
	assert.Equal(t, auth.RoleOperator, user.Role)
}

func TestProcessLogin_DisabledUser(t *testing.T) {
	store := newMockUserStore()
	existingUser := &auth.User{
		ID:           "disabled-id",
		Username:     "disableduser",
		Role:         auth.RoleViewer,
		AuthProvider: "oidc",
		OIDCIssuer:   "https://issuer.example.com",
		OIDCSubject:  "sub123",
		IsDisabled:   true,
	}
	_ = store.Create(context.Background(), existingUser)

	svc := New(store, Config{
		Issuer:      "https://issuer.example.com",
		AutoSignup:  true,
		DefaultRole: auth.RoleViewer,
	})

	claims := OIDCClaims{
		Subject:           "sub123",
		Email:             "user@example.com",
		PreferredUsername: "testuser",
	}

	user, isNew, err := svc.ProcessLogin(context.Background(), claims)
	assert.ErrorIs(t, err, ErrUserDisabled)
	assert.Nil(t, user)
	assert.False(t, isNew)
}

func TestProcessLogin_AutoSignupDisabled(t *testing.T) {
	store := newMockUserStore()
	svc := New(store, Config{
		Issuer:      "https://issuer.example.com",
		AutoSignup:  false, // Disabled
		DefaultRole: auth.RoleViewer,
	})

	claims := OIDCClaims{
		Subject:           "sub123",
		Email:             "user@example.com",
		PreferredUsername: "testuser",
	}

	user, isNew, err := svc.ProcessLogin(context.Background(), claims)
	assert.ErrorIs(t, err, ErrAutoSignupDisabled)
	assert.Nil(t, user)
	assert.False(t, isNew)
}

func TestProcessLogin_EmailRequired(t *testing.T) {
	store := newMockUserStore()
	svc := New(store, Config{
		Issuer:      "https://issuer.example.com",
		AutoSignup:  true,
		DefaultRole: auth.RoleViewer,
	})

	claims := OIDCClaims{
		Subject:           "sub123",
		Email:             "", // No email
		PreferredUsername: "testuser",
	}

	user, isNew, err := svc.ProcessLogin(context.Background(), claims)
	assert.ErrorIs(t, err, ErrEmailRequired)
	assert.Nil(t, user)
	assert.False(t, isNew)
}

func TestProcessLogin_EmailNotAllowed_Domain(t *testing.T) {
	store := newMockUserStore()
	svc := New(store, Config{
		Issuer:         "https://issuer.example.com",
		AutoSignup:     true,
		DefaultRole:    auth.RoleViewer,
		AllowedDomains: []string{"company.com", "partner.org"},
	})

	claims := OIDCClaims{
		Subject:           "sub123",
		Email:             "user@unauthorized.com",
		PreferredUsername: "testuser",
	}

	user, isNew, err := svc.ProcessLogin(context.Background(), claims)
	assert.ErrorIs(t, err, ErrEmailNotAllowed)
	assert.Nil(t, user)
	assert.False(t, isNew)
}

func TestProcessLogin_EmailAllowed_Domain(t *testing.T) {
	store := newMockUserStore()
	svc := New(store, Config{
		Issuer:         "https://issuer.example.com",
		AutoSignup:     true,
		DefaultRole:    auth.RoleViewer,
		AllowedDomains: []string{"company.com", "partner.org"},
	})

	claims := OIDCClaims{
		Subject:           "sub123",
		Email:             "user@company.com",
		PreferredUsername: "testuser",
	}

	user, isNew, err := svc.ProcessLogin(context.Background(), claims)
	require.NoError(t, err)
	assert.True(t, isNew)
	assert.NotNil(t, user)
}

func TestProcessLogin_EmailAllowed_Whitelist(t *testing.T) {
	store := newMockUserStore()
	svc := New(store, Config{
		Issuer:         "https://issuer.example.com",
		AutoSignup:     true,
		DefaultRole:    auth.RoleViewer,
		AllowedDomains: []string{"company.com"}, // contractor@gmail.com domain not allowed
		Whitelist:      []string{"contractor@gmail.com"},
	})

	claims := OIDCClaims{
		Subject:           "sub123",
		Email:             "contractor@gmail.com",
		PreferredUsername: "contractor",
	}

	// Whitelist takes precedence over AllowedDomains
	user, isNew, err := svc.ProcessLogin(context.Background(), claims)
	require.NoError(t, err)
	assert.True(t, isNew)
	assert.NotNil(t, user)
	assert.Equal(t, "contractor", user.Username)
}

func TestGenerateUniqueUsername_NoConflict(t *testing.T) {
	store := newMockUserStore()
	svc := New(store, Config{
		Issuer:      "https://issuer.example.com",
		AutoSignup:  true,
		DefaultRole: auth.RoleViewer,
	})

	claims := OIDCClaims{
		Subject:           "sub123",
		Email:             "john.doe@example.com",
		PreferredUsername: "johndoe",
	}

	username := svc.generateUniqueUsername(context.Background(), claims)
	assert.Equal(t, "johndoe", username)
}

func TestGenerateUniqueUsername_ConflictWithOIDCUser(t *testing.T) {
	store := newMockUserStore()
	// Create existing OIDC user with same username
	existingUser := &auth.User{
		ID:           "existing-id",
		Username:     "johndoe",
		AuthProvider: "oidc",
		OIDCIssuer:   "https://other-issuer.example.com",
		OIDCSubject:  "other-sub",
	}
	_ = store.Create(context.Background(), existingUser)

	svc := New(store, Config{
		Issuer:      "https://issuer.example.com",
		AutoSignup:  true,
		DefaultRole: auth.RoleViewer,
	})

	claims := OIDCClaims{
		Subject:           "sub123",
		Email:             "john.doe@example.com",
		PreferredUsername: "johndoe",
	}

	username := svc.generateUniqueUsername(context.Background(), claims)
	assert.Equal(t, "johndoe2", username)
}

func TestGenerateUniqueUsername_ConflictWithBuiltinUser(t *testing.T) {
	store := newMockUserStore()
	// Create existing builtin user with same username
	existingUser := &auth.User{
		ID:           "existing-id",
		Username:     "johndoe",
		AuthProvider: "builtin", // This is a builtin user
	}
	_ = store.Create(context.Background(), existingUser)

	svc := New(store, Config{
		Issuer:      "https://issuer.example.com",
		AutoSignup:  true,
		DefaultRole: auth.RoleViewer,
	})

	claims := OIDCClaims{
		Subject:           "sub123",
		Email:             "john.doe@example.com",
		PreferredUsername: "johndoe",
	}

	// Should add _sso suffix to differentiate from builtin user
	username := svc.generateUniqueUsername(context.Background(), claims)
	assert.Equal(t, "johndoe_sso", username)
}

func TestGenerateUniqueUsername_FallbackToEmail(t *testing.T) {
	store := newMockUserStore()
	svc := New(store, Config{
		Issuer:      "https://issuer.example.com",
		AutoSignup:  true,
		DefaultRole: auth.RoleViewer,
	})

	claims := OIDCClaims{
		Subject:           "sub123",
		Email:             "john.doe@example.com",
		PreferredUsername: "", // No preferred username
	}

	username := svc.generateUniqueUsername(context.Background(), claims)
	assert.Equal(t, "john_doe", username) // Email local part sanitized
}

func TestGenerateUniqueUsername_FallbackToSubject(t *testing.T) {
	store := newMockUserStore()

	// Create users to exhaust all candidates including _sso variants
	// First, block the base and _sso versions
	for _, name := range []string{"john_doe", "john_doe_sso"} {
		_ = store.Create(context.Background(), &auth.User{
			ID:           name + "-id",
			Username:     name,
			AuthProvider: "builtin", // Use builtin to trigger _sso logic
		})
	}
	// Block all numbered OIDC variants (john_doe2 through john_doe99)
	for i := 2; i <= 99; i++ {
		name := fmt.Sprintf("john_doe%d", i)
		_ = store.Create(context.Background(), &auth.User{
			ID:           name + "-id",
			Username:     name,
			AuthProvider: "oidc",
		})
	}
	// Block all numbered _sso variants (john_doe_sso2 through john_doe_sso99)
	for i := 2; i <= 99; i++ {
		name := fmt.Sprintf("john_doe_sso%d", i)
		_ = store.Create(context.Background(), &auth.User{
			ID:           name + "-id",
			Username:     name,
			AuthProvider: "oidc",
		})
	}

	svc := New(store, Config{
		Issuer:      "https://issuer.example.com",
		AutoSignup:  true,
		DefaultRole: auth.RoleViewer,
	})

	claims := OIDCClaims{
		Subject:           "abcd1234efgh5678",
		Email:             "john.doe@example.com",
		PreferredUsername: "", // No preferred username
	}

	username := svc.generateUniqueUsername(context.Background(), claims)
	assert.Equal(t, "user_abcd1234", username) // First 8 chars of subject
}

func TestSanitizeUsername(t *testing.T) {
	svc := &Service{}

	tests := []struct {
		input    string
		expected string
	}{
		{"johndoe", "johndoe"},
		{"John.Doe", "john_doe"},
		{"john-doe", "john_doe"},
		{"john doe", "john_doe"},
		{"john@doe", "johndoe"},
		{"UPPERCASE", "uppercase"},
		{"user123", "user123"},
		{"___test___", "test"},
		{"john.doe.smith", "john_doe_smith"},
		{"日本語", ""}, // Non-ASCII removed
		{"test!@#$%", "test"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := svc.sanitizeUsername(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsEmailAllowed(t *testing.T) {
	tests := []struct {
		name           string
		allowedDomains []string
		whitelist      []string
		email          string
		expected       bool
	}{
		{
			name:     "no restrictions - all allowed",
			email:    "user@any.com",
			expected: true,
		},
		{
			name:           "domain allowed",
			allowedDomains: []string{"company.com"},
			email:          "user@company.com",
			expected:       true,
		},
		{
			name:           "domain not allowed",
			allowedDomains: []string{"company.com"},
			email:          "user@other.com",
			expected:       false,
		},
		{
			name:           "whitelist takes precedence",
			allowedDomains: []string{"company.com"},
			whitelist:      []string{"special@other.com"},
			email:          "special@other.com",
			expected:       true,
		},
		{
			name:           "case insensitive domain",
			allowedDomains: []string{"Company.COM"},
			email:          "user@company.com",
			expected:       true,
		},
		{
			name:      "case insensitive whitelist",
			whitelist: []string{"User@Company.COM"},
			email:     "user@company.com",
			expected:  true,
		},
		{
			name:           "multiple domains",
			allowedDomains: []string{"company.com", "partner.org"},
			email:          "user@partner.org",
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &Service{
				config: Config{
					AllowedDomains: tt.allowedDomains,
					Whitelist:      tt.whitelist,
				},
			}
			result := svc.isEmailAllowed(tt.email)
			assert.Equal(t, tt.expected, result)
		})
	}
}
