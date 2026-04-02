// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"context"
	"errors"
	"net/http"
	"time"
)

var ErrRemoteContextNotFound = errors.New("remote context not found")

// RemoteContextInfo contains resolved information about a remote CLI context.
type RemoteContextInfo struct {
	Name          string
	Description   string
	APIBaseURL    string
	AuthToken     string
	SkipTLSVerify bool
	Timeout       time.Duration
}

// ApplyAuth adds the Bearer token header to the request.
// If the token is empty, no header is set.
func (n *RemoteContextInfo) ApplyAuth(req *http.Request) {
	if n.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+n.AuthToken)
	}
}

// RemoteContextResolver resolves remote CLI contexts for remote agent tools.
type RemoteContextResolver interface {
	GetByName(ctx context.Context, name string) (RemoteContextInfo, error)

	ListRemoteContexts(ctx context.Context) ([]RemoteContextInfo, error)
}
