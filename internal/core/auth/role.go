// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package auth

import "fmt"

// Role represents a user's role in the system.
// Roles determine what actions a user can perform.
type Role string

const (
	// RoleAdmin has full access to all resources and system settings.
	RoleAdmin Role = "admin"
	// RoleEditor can create, edit, delete DAGs and run them.
	RoleEditor Role = "editor"
	// RoleViewer can only view DAGs and execution history (read-only).
	RoleViewer Role = "viewer"
)

// allRoles contains all valid roles for iteration and validation.
var allRoles = []Role{RoleAdmin, RoleEditor, RoleViewer}

// AllRoles returns a copy of all valid roles.
func AllRoles() []Role {
	roles := make([]Role, len(allRoles))
	copy(roles, allRoles)
	return roles
}

// Valid returns true if the role is a known valid role.
func (r Role) Valid() bool {
	switch r {
	case RoleAdmin, RoleEditor, RoleViewer:
		return true
	}
	return false
}

// String returns the string representation of the role.
func (r Role) String() string {
	return string(r)
}

// CanWrite returns true if the role can create, edit, or delete resources.
func (r Role) CanWrite() bool {
	return r == RoleAdmin || r == RoleEditor
}

// IsAdmin returns true if the role has administrative privileges.
func (r Role) IsAdmin() bool {
	return r == RoleAdmin
}

// ParseRole converts a string to a Role.
// Returns an error if the string is not a valid role.
func ParseRole(s string) (Role, error) {
	role := Role(s)
	if !role.Valid() {
		return "", fmt.Errorf("invalid role: %q, must be one of: admin, editor, viewer", s)
	}
	return role, nil
}
