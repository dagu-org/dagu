package remotenode

import (
	"context"
	"errors"
)

// Common errors for remote node store operations.
var (
	// ErrRemoteNodeNotFound is returned when a remote node cannot be found.
	ErrRemoteNodeNotFound = errors.New("remote node not found")
	// ErrRemoteNodeAlreadyExists is returned when attempting to create a remote node
	// with a name that already exists.
	ErrRemoteNodeAlreadyExists = errors.New("remote node with this name already exists")
	// ErrInvalidRemoteNodeName is returned when the remote node name is invalid.
	ErrInvalidRemoteNodeName = errors.New("invalid remote node name")
	// ErrInvalidRemoteNodeID is returned when the remote node ID is invalid.
	ErrInvalidRemoteNodeID = errors.New("invalid remote node ID")
)

// Store defines the interface for remote node persistence operations.
// Implementations must be safe for concurrent use.
type Store interface {
	// Create stores a new remote node.
	// Returns ErrRemoteNodeAlreadyExists if a node with the same name exists.
	Create(ctx context.Context, node *RemoteNode) error

	// GetByID retrieves a remote node by its unique ID.
	// Returns ErrRemoteNodeNotFound if the node does not exist.
	GetByID(ctx context.Context, id string) (*RemoteNode, error)

	// GetByName retrieves a remote node by its name.
	// Returns ErrRemoteNodeNotFound if the node does not exist.
	GetByName(ctx context.Context, name string) (*RemoteNode, error)

	// List returns all remote nodes in the store.
	List(ctx context.Context) ([]*RemoteNode, error)

	// Update modifies an existing remote node.
	// Returns ErrRemoteNodeNotFound if the node does not exist.
	Update(ctx context.Context, node *RemoteNode) error

	// Delete removes a remote node by its ID.
	// Returns ErrRemoteNodeNotFound if the node does not exist.
	Delete(ctx context.Context, id string) error
}
