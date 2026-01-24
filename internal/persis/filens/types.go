// Package filens provides a file-based implementation of namespace storage.
package filens

import (
	"errors"
	"regexp"
	"time"
)

const (
	// MinNameLength is the minimum length for a namespace name.
	MinNameLength = 1
	// MaxNameLength is the maximum length for a namespace name.
	MaxNameLength = 63
)

var (
	// ErrNamespaceNotFound is returned when a namespace does not exist.
	ErrNamespaceNotFound = errors.New("namespace not found")
	// ErrNamespaceAlreadyExists is returned when attempting to create a namespace that already exists.
	ErrNamespaceAlreadyExists = errors.New("namespace already exists")
	// ErrInvalidNamespaceID is returned when an invalid namespace ID is provided.
	ErrInvalidNamespaceID = errors.New("invalid namespace ID")
	// ErrInvalidNamespaceName is returned when an invalid namespace name is provided.
	ErrInvalidNamespaceName = errors.New("invalid namespace name")
	// ErrReservedNamespaceName is returned when attempting to use a reserved namespace name.
	ErrReservedNamespaceName = errors.New("reserved namespace name")
	// ErrNamespaceNotEmpty is returned when attempting to delete a non-empty namespace.
	ErrNamespaceNotEmpty = errors.New("namespace is not empty")
)

// reservedNames contains namespace names that cannot be used.
var reservedNames = map[string]bool{
	"system":   true,
	"admin":    true,
	"api":      true,
	"internal": true,
	"global":   true,
}

// namespaceNamePattern matches valid namespace names:
// - Starts with a lowercase letter
// - Contains only lowercase alphanumeric, hyphens, underscores
// - Does not end with hyphen or underscore
var namespaceNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*[a-z0-9]$|^[a-z]$`)

// Namespace represents a namespace in the system.
// Namespaces provide isolation boundaries for DAGs and their resources.
type Namespace struct {
	// ID is the unique identifier (UUID) for the namespace.
	// This is used for storage paths to enable renaming without moving files.
	ID string `json:"id"`

	// Name is the human-readable name of the namespace.
	// Must be unique across all namespaces.
	Name string `json:"name"`

	// DisplayName is the display name shown in the UI.
	DisplayName string `json:"displayName,omitempty"`

	// Description is an optional description of the namespace.
	Description string `json:"description,omitempty"`

	// CreatedAt is the timestamp when the namespace was created.
	CreatedAt time.Time `json:"createdAt"`

	// CreatedBy is the ID of the user who created the namespace.
	CreatedBy string `json:"createdBy,omitempty"`

	// UpdatedAt is the timestamp when the namespace was last updated.
	UpdatedAt time.Time `json:"updatedAt"`
}

// ValidateName checks if a namespace name is valid.
// Returns nil if valid, or an appropriate error.
func ValidateName(name string) error {
	if name == "" {
		return ErrInvalidNamespaceName
	}
	if len(name) < MinNameLength || len(name) > MaxNameLength {
		return ErrInvalidNamespaceName
	}
	if reservedNames[name] {
		return ErrReservedNamespaceName
	}
	if !namespaceNamePattern.MatchString(name) {
		return ErrInvalidNamespaceName
	}
	return nil
}

// IsReservedName checks if a name is reserved.
func IsReservedName(name string) bool {
	return reservedNames[name]
}
