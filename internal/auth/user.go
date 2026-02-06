package auth

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system.
type User struct {
	// ID is the unique identifier for the user (UUID).
	ID string `json:"id"`
	// Username is the unique login name.
	Username string `json:"username"`
	// PasswordHash is the bcrypt hash of the password.
	// Excluded from JSON serialization for security.
	PasswordHash string `json:"-"`
	// Role determines the user's permissions.
	Role Role `json:"role"`
	// CreatedAt is the timestamp when the user was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the user was last modified.
	UpdatedAt time.Time `json:"updated_at"`

	// AuthProvider indicates how the user was created: "builtin" or "oidc".
	// Empty string is treated as "builtin" for backward compatibility.
	AuthProvider string `json:"auth_provider,omitempty"`
	// OIDCIssuer is the OIDC provider issuer URL (set when AuthProvider is "oidc").
	OIDCIssuer string `json:"oidc_issuer,omitempty"`
	// OIDCSubject is the unique subject identifier from the OIDC provider.
	OIDCSubject string `json:"oidc_subject,omitempty"`
	// IsDisabled indicates if the user account is disabled.
	// Disabled users cannot log in.
	IsDisabled bool `json:"is_disabled,omitempty"`

	// NamespaceRoles maps namespace names to per-namespace roles.
	// A user's effective role in a namespace is the higher of their
	// global Role and their namespace-specific role.
	NamespaceRoles map[string]Role `json:"namespace_roles,omitempty"`
}

// NewUser creates a User with a new UUID and sets CreatedAt and UpdatedAt to the current UTC time.
// The provided username, passwordHash, and role are assigned to the corresponding fields.
func NewUser(username string, passwordHash string, role Role) *User {
	now := time.Now().UTC()
	return &User{
		ID:           uuid.New().String(),
		Username:     username,
		PasswordHash: passwordHash,
		Role:         role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// UserForStorage is used for JSON serialization to persistent storage.
// It includes the password hash which is excluded from the regular User JSON.
type UserForStorage struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	AuthProvider string    `json:"auth_provider,omitempty"`
	OIDCIssuer   string    `json:"oidc_issuer,omitempty"`
	OIDCSubject    string          `json:"oidc_subject,omitempty"`
	IsDisabled     bool            `json:"is_disabled,omitempty"`
	NamespaceRoles map[string]Role `json:"namespace_roles,omitempty"`
}

// ToStorage converts a User to UserForStorage for persistence.
func (u *User) ToStorage() *UserForStorage {
	return &UserForStorage{
		ID:             u.ID,
		Username:       u.Username,
		PasswordHash:   u.PasswordHash,
		Role:           u.Role,
		CreatedAt:      u.CreatedAt,
		UpdatedAt:      u.UpdatedAt,
		AuthProvider:   u.AuthProvider,
		OIDCIssuer:     u.OIDCIssuer,
		OIDCSubject:    u.OIDCSubject,
		IsDisabled:     u.IsDisabled,
		NamespaceRoles: u.NamespaceRoles,
	}
}

// ToUser converts UserForStorage back to User.
func (s *UserForStorage) ToUser() *User {
	return &User{
		ID:             s.ID,
		Username:       s.Username,
		PasswordHash:   s.PasswordHash,
		Role:           s.Role,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
		AuthProvider:   s.AuthProvider,
		OIDCIssuer:     s.OIDCIssuer,
		OIDCSubject:    s.OIDCSubject,
		IsDisabled:     s.IsDisabled,
		NamespaceRoles: s.NamespaceRoles,
	}
}

// IsOIDCUser returns true if the user was created via OIDC auto-signup.
func (u *User) IsOIDCUser() bool {
	return u.AuthProvider == "oidc"
}

// EffectiveRoleInNamespace returns the user's effective role in the given namespace.
// The effective role is the higher of the global role and the namespace-specific role.
// A user with no global role and no namespace role has no access (empty Role returned).
func (u *User) EffectiveRoleInNamespace(namespace string) Role {
	nsRole, hasNS := u.NamespaceRoles[namespace]
	if hasNS {
		return HigherRole(u.Role, nsRole)
	}
	return u.Role
}

// HasNamespaceAccess returns true if the user has any valid role in the given namespace.
// A global admin always has access. Otherwise the user needs either a global role or
// a namespace-specific role.
func (u *User) HasNamespaceAccess(namespace string) bool {
	effective := u.EffectiveRoleInNamespace(namespace)
	return effective.Valid()
}
