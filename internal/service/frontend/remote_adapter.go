package frontend

import (
	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/remotenode"
)

// newRemoteNodeAdapter creates a RemoteNodeResolver from a remotenode.Resolver.
// Returns nil if the resolver is nil (so tool factories hide the tools).
func newRemoteNodeAdapter(resolver *remotenode.Resolver) agent.RemoteNodeResolver {
	if resolver == nil {
		return nil
	}
	return &agent.RemoteNodeResolverAdapter{Resolver: resolver}
}
