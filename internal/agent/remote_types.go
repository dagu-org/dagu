package agent

import (
	"context"
	"net/http"
	"time"
)

// RemoteNodeInfo contains resolved information about a remote Dagu node
// for use by agent tools. This type is intentionally separate from
// remotenode.RemoteNode to avoid circular dependencies.
type RemoteNodeInfo struct {
	Name          string
	Description   string
	APIBaseURL    string
	AuthToken     string
	SkipTLSVerify bool
	Timeout       time.Duration
}

// ApplyAuth adds the Bearer token header to the request.
// If the token is empty, no header is set.
func (n *RemoteNodeInfo) ApplyAuth(req *http.Request) {
	if n.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+n.AuthToken)
	}
}

// RemoteNodeResolver resolves remote node information for agent tools.
// Implementations filter to only token-auth nodes (basic-auth nodes
// cannot be used for remote agent operations).
type RemoteNodeResolver interface {
	// GetByName returns a remote node by name.
	// Returns an error if the node is not found or not token-auth.
	GetByName(ctx context.Context, name string) (RemoteNodeInfo, error)

	// ListTokenAuthNodes returns all remote nodes that use token authentication.
	ListTokenAuthNodes(ctx context.Context) ([]RemoteNodeInfo, error)
}
