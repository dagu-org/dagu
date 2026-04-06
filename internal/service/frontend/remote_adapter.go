// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package frontend

import (
	"context"
	"fmt"

	"github.com/dagucloud/dagu/internal/agent"
	"github.com/dagucloud/dagu/internal/remotenode"
)

type remoteContextResolverAdapter struct {
	resolver *remotenode.Resolver
}

// newRemoteNodeAdapter creates a RemoteContextResolver from a remotenode.Resolver.
// Returns nil if the resolver is nil (so tool factories hide the tools).
func newRemoteNodeAdapter(resolver *remotenode.Resolver) agent.RemoteContextResolver {
	if resolver == nil {
		return nil
	}
	return &remoteContextResolverAdapter{resolver: resolver}
}

func (a *remoteContextResolverAdapter) GetByName(ctx context.Context, name string) (agent.RemoteContextInfo, error) {
	node, err := a.resolver.GetByName(ctx, name)
	if err != nil {
		return agent.RemoteContextInfo{}, fmt.Errorf("context %q not found: %w", name, err)
	}
	if node.AuthType != remotenode.AuthTypeToken {
		return agent.RemoteContextInfo{}, fmt.Errorf("context %q uses %s auth (only token auth is supported)", name, node.AuthType)
	}
	return agent.RemoteContextInfo{
		Name:          node.Name,
		Description:   node.Description,
		APIBaseURL:    node.APIBaseURL,
		AuthToken:     node.AuthToken,
		SkipTLSVerify: node.SkipTLSVerify,
		Timeout:       node.Timeout,
	}, nil
}

func (a *remoteContextResolverAdapter) ListRemoteContexts(ctx context.Context) ([]agent.RemoteContextInfo, error) {
	nodes, err := a.resolver.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]agent.RemoteContextInfo, 0, len(nodes))
	for _, node := range nodes {
		if node.AuthType != remotenode.AuthTypeToken {
			continue
		}
		out = append(out, agent.RemoteContextInfo{
			Name:          node.Name,
			Description:   node.Description,
			APIBaseURL:    node.APIBaseURL,
			AuthToken:     node.AuthToken,
			SkipTLSVerify: node.SkipTLSVerify,
			Timeout:       node.Timeout,
		})
	}
	return out, nil
}
