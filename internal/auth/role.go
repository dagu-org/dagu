package auth

import "fmt"

// Role represents a user's role in the system.
// Roles determine what actions a user can perform.
//
// Role hierarchy (most to least privileged):
//   - admin: Full system access including user management
//   - manager: Can create, edit, delete, run, and stop DAGs
//   - operator: Can run and stop DAGs (execute only)
//   - viewer: Read-only access to DAGs and status
type Role string

const (
	// RoleAdmin has full access to all resources including user management.
	RoleAdmin Role = "admin"
	// RoleManager can create, edit, delete, run, and stop DAGs.
	RoleManager Role = "manager"
	// RoleOperator can run and stop DAGs (execute only, no edit).
	RoleOperator Role = "operator"
	// RoleViewer can only view DAGs and execution history (read-only).
	RoleViewer Role = "viewer"
)

// allRoles contains all valid roles for iteration and validation.
var allRoles = []Role{RoleAdmin, RoleManager, RoleOperator, RoleViewer}

// AllRoles returns a copy of all valid roles.
func AllRoles() []Role {
	roles := make([]Role, len(allRoles))
	copy(roles, allRoles)
	return roles
}

// Valid returns true if the role is a known valid role.
func (r Role) Valid() bool {
	switch r {
	case RoleAdmin, RoleManager, RoleOperator, RoleViewer:
		return true
	}
	return false
}

// String returns the string representation of the role.
func (r Role) String() string {
	return string(r)
}

// CanWrite returns true if the role can create, edit, or delete DAGs.
func (r Role) CanWrite() bool {
	return r == RoleAdmin || r == RoleManager
}

// CanExecute returns true if the role can run or stop DAGs.
func (r Role) CanExecute() bool {
	return r == RoleAdmin || r == RoleManager || r == RoleOperator
}

// IsAdmin returns true if the role has administrative privileges (user management).
func (r Role) IsAdmin() bool {
	return r == RoleAdmin
}

// ParseRole converts a string to a Role.
// ParseRole converts the input string to a Role and verifies it is one of the known roles.
// If the input is not "admin", "manager", "operator", or "viewer", it returns an error describing the valid options.
func ParseRole(s string) (Role, error) {
	role := Role(s)
	if !role.Valid() {
		return "", fmt.Errorf("invalid role: %q, must be one of: admin, manager, operator, viewer", s)
	}
	return role, nil
}
