package frontend

import (
	"context"
	"fmt"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/remotenode"
)

// remoteNodeAdapter adapts *remotenode.Resolver to agent.RemoteNodeResolver.
// It filters to token-auth nodes only, since basic-auth nodes cannot be used
// for remote agent operations.
type remoteNodeAdapter struct {
	resolver *remotenode.Resolver
}

// newRemoteNodeAdapter creates an adapter from a remotenode.Resolver.
// Returns nil if the resolver has no token-auth nodes (so tool factories hide the tools).
func newRemoteNodeAdapter(resolver *remotenode.Resolver) agent.RemoteNodeResolver {
	if resolver == nil {
		return nil
	}
	return &remoteNodeAdapter{resolver: resolver}
}

// GetByName returns a remote node by name, rejecting non-token-auth nodes.
func (a *remoteNodeAdapter) GetByName(ctx context.Context, name string) (agent.RemoteNodeInfo, error) {
	node, err := a.resolver.GetByName(ctx, name)
	if err != nil {
		return agent.RemoteNodeInfo{}, fmt.Errorf("node %q not found: %w", name, err)
	}

	if node.AuthType != remotenode.AuthTypeToken {
		return agent.RemoteNodeInfo{}, fmt.Errorf("node %q uses %s auth (only token auth is supported for remote agent)", name, node.AuthType)
	}

	return toRemoteNodeInfo(node), nil
}

// ListTokenAuthNodes returns all remote nodes that use token authentication.
func (a *remoteNodeAdapter) ListTokenAuthNodes(ctx context.Context) ([]agent.RemoteNodeInfo, error) {
	all, err := a.resolver.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	var result []agent.RemoteNodeInfo
	for _, rn := range all {
		if rn.AuthType == remotenode.AuthTypeToken {
			result = append(result, toRemoteNodeInfo(rn.RemoteNode))
		}
	}
	return result, nil
}

// toRemoteNodeInfo converts a domain RemoteNode to an agent RemoteNodeInfo.
func toRemoteNodeInfo(n *remotenode.RemoteNode) agent.RemoteNodeInfo {
	return agent.RemoteNodeInfo{
		Name:          n.Name,
		Description:   n.Description,
		APIBaseURL:    n.APIBaseURL,
		AuthToken:     n.AuthToken,
		SkipTLSVerify: n.SkipTLSVerify,
		Timeout:       n.Timeout,
	}
}
