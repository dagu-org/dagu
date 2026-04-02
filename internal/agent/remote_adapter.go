// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/clicontext"
)

// RemoteContextResolverAdapter adapts the CLI context store to RemoteContextResolver.
type RemoteContextResolverAdapter struct {
	Store *clicontext.Store
}

func (a *RemoteContextResolverAdapter) GetByName(ctx context.Context, name string) (RemoteContextInfo, error) {
	if a == nil || a.Store == nil {
		return RemoteContextInfo{}, fmt.Errorf("remote context store is not configured")
	}
	item, err := a.Store.Get(ctx, name)
	if err != nil {
		return RemoteContextInfo{}, fmt.Errorf("context %q not found: %w", name, err)
	}
	if item.Name == clicontext.LocalContextName {
		return RemoteContextInfo{}, fmt.Errorf("context %q is local and cannot be used for remote agent execution", name)
	}
	return toRemoteContextInfo(item), nil
}

func (a *RemoteContextResolverAdapter) ListRemoteContexts(ctx context.Context) ([]RemoteContextInfo, error) {
	if a == nil || a.Store == nil {
		return nil, nil
	}
	items, err := a.Store.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RemoteContextInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toRemoteContextInfo(item))
	}
	return out, nil
}

func toRemoteContextInfo(item *clicontext.Context) RemoteContextInfo {
	timeout := time.Duration(item.TimeoutSeconds) * time.Second
	return RemoteContextInfo{
		Name:          item.Name,
		Description:   item.Description,
		APIBaseURL:    item.ServerURL,
		AuthToken:     item.APIKey,
		SkipTLSVerify: item.SkipTLSVerify,
		Timeout:       timeout,
	}
}
