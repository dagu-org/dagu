package agent

import (
	"context"
	"fmt"

	"github.com/dagu-org/dagu/internal/remotenode"
)

// RemoteNodeResolverAdapter adapts *remotenode.Resolver to RemoteNodeResolver.
// It filters to token-auth nodes only, since basic-auth nodes cannot be
// used for remote agent operations.
type RemoteNodeResolverAdapter struct {
	Resolver *remotenode.Resolver
}

// GetByName returns a remote node by name, rejecting non-token-auth nodes.
func (a *RemoteNodeResolverAdapter) GetByName(ctx context.Context, name string) (RemoteNodeInfo, error) {
	node, err := a.Resolver.GetByName(ctx, name)
	if err != nil {
		return RemoteNodeInfo{}, fmt.Errorf("node %q not found: %w", name, err)
	}
	if node.AuthType != remotenode.AuthTypeToken {
		return RemoteNodeInfo{}, fmt.Errorf(
			"node %q uses %s auth (only token auth is supported for remote agent)", name, node.AuthType,
		)
	}
	return toRemoteNodeInfo(node), nil
}

// ListTokenAuthNodes returns all remote nodes that use token authentication.
func (a *RemoteNodeResolverAdapter) ListTokenAuthNodes(ctx context.Context) ([]RemoteNodeInfo, error) {
	all, err := a.Resolver.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	var result []RemoteNodeInfo
	for _, rn := range all {
		if rn.AuthType == remotenode.AuthTypeToken {
			result = append(result, toRemoteNodeInfo(rn.RemoteNode))
		}
	}
	return result, nil
}

// toRemoteNodeInfo converts a domain RemoteNode to an agent RemoteNodeInfo.
func toRemoteNodeInfo(n *remotenode.RemoteNode) RemoteNodeInfo {
	return RemoteNodeInfo{
		Name:          n.Name,
		Description:   n.Description,
		APIBaseURL:    n.APIBaseURL,
		AuthToken:     n.AuthToken,
		SkipTLSVerify: n.SkipTLSVerify,
		Timeout:       n.Timeout,
	}
}
