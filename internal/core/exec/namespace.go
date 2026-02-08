package exec

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/dagu-org/dagu/internal/core"
)

// Errors for namespace operations
var (
	ErrNamespaceNotFound      = errors.New("namespace not found")
	ErrNamespaceAlreadyExists = errors.New("namespace already exists")
	ErrNamespaceHashCollision = errors.New("namespace hash collision: two different names produced the same ID")
)

// NamespaceStore is an interface for managing namespace storage.
type NamespaceStore interface {
	// Create persists a new namespace and returns it.
	Create(ctx context.Context, opts CreateNamespaceOptions) (*Namespace, error)
	// Delete removes a namespace by name.
	Delete(ctx context.Context, name string) error
	// Get retrieves a namespace by name.
	Get(ctx context.Context, name string) (*Namespace, error)
	// List returns all namespaces.
	List(ctx context.Context) ([]*Namespace, error)
	// Resolve returns the ID for a given namespace name.
	Resolve(ctx context.Context, name string) (string, error)
	// Update applies partial updates to an existing namespace.
	Update(ctx context.Context, name string, opts UpdateNamespaceOptions) (*Namespace, error)
}

// CreateNamespaceOptions contains parameters for creating a namespace.
type CreateNamespaceOptions struct {
	Name        string
	Description string
	Defaults    NamespaceDefaults
	GitSync     NamespaceGitSync
}

// UpdateNamespaceOptions contains parameters for updating a namespace.
// Nil pointer fields are left unchanged; non-nil fields are applied.
type UpdateNamespaceOptions struct {
	Description    *string
	Defaults       *NamespaceDefaults
	BaseConfig     *core.DAG
	BaseConfigYAML *string
	GitSync        *NamespaceGitSync
}

// Namespace represents a first-class isolation boundary in Dagu.
type Namespace struct {
	// Name is the human-readable namespace name.
	Name string `json:"name"`
	// ID is a 4-character hex prefix derived from SHA256 of the name.
	ID string `json:"id"`
	// CreatedAt is the timestamp when the namespace was created.
	CreatedAt time.Time `json:"createdAt"`
	// Description is an optional description of the namespace.
	Description string `json:"description,omitempty"`
	// BaseConfig is an optional base DAG configuration applied to all DAGs in this namespace.
	BaseConfig *core.DAG `json:"baseConfig,omitempty"`
	// BaseConfigYAML is the raw YAML source for the base config, preserved for display in the UI.
	BaseConfigYAML string `json:"baseConfigYAML,omitempty"`
	// Defaults contains default values for DAGs in this namespace.
	Defaults NamespaceDefaults `json:"defaults,omitzero"`
	// GitSync contains git sync configuration for this namespace.
	GitSync NamespaceGitSync `json:"gitSync,omitzero"`
}

// NsDir is the directory name grouping namespace-scoped subdirectories.
const NsDir = "ns"

// NamespaceDir returns the root directory for a namespace under the given base:
//
//	{baseDir}/ns/{nsID}
func NamespaceDir(baseDir, nsID string) string {
	return filepath.Join(baseDir, NsDir, nsID)
}

// NamespaceHasDAGs checks if a namespace DAG directory contains any YAML files.
func NamespaceHasDAGs(dagDir string) (bool, error) {
	entries, err := os.ReadDir(dagDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext == ".yaml" || ext == ".yml" {
			return true, nil
		}
	}
	return false, nil
}

// NamespaceDefaults contains default values for DAGs in a namespace.
type NamespaceDefaults struct {
	// Queue is the default queue name for DAGs in this namespace.
	Queue string `json:"queue,omitempty"`
	// WorkingDir is the default working directory for DAGs in this namespace.
	WorkingDir string `json:"workingDir,omitempty"`
}

// NamespaceGitSync contains git sync configuration for a namespace.
type NamespaceGitSync struct {
	// RemoteURL is the git remote URL.
	RemoteURL string `json:"remoteURL,omitempty"`
	// Branch is the git branch to sync.
	Branch string `json:"branch,omitempty"`
	// SSHKeyRef is a reference to the SSH key for authentication.
	SSHKeyRef string `json:"sshKeyRef,omitempty"`
	// Path is the subdirectory within the repo for this namespace.
	Path string `json:"path,omitempty"`
	// AutoSyncInterval is the interval for automatic syncing.
	AutoSyncInterval string `json:"autoSyncInterval,omitempty"`
}
