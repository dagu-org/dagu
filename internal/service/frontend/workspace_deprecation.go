// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package frontend

import (
	"context"
	"log/slog"

	"github.com/dagucloud/dagu/v2/internal/cmn/config"
	"github.com/dagucloud/dagu/v2/internal/cmn/logger"
	"github.com/dagucloud/dagu/v2/internal/cmn/logger/tag"
	"github.com/dagucloud/dagu/v2/internal/persis"
	authservice "github.com/dagucloud/dagu/v2/internal/service/auth"
	"github.com/dagucloud/dagu/v2/internal/workspace/deprecation"
)

// warnWorkspaceDeprecation logs every workspace setting in use that stops
// applying when workspaces are removed in v3.0, in one warning.
func (srv *Server) warnWorkspaceDeprecation(ctx context.Context) {
	items, err := deprecation.Find(ctx, srv.workspaceDeprecation)
	if err != nil {
		logger.Warn(ctx, "Failed to read some workspace settings", tag.Error(err))
	}
	if len(items) == 0 {
		return
	}
	settings := make([]string, 0, len(items))
	for _, item := range items {
		settings = append(settings, item.String())
	}
	logger.Warn(ctx, deprecation.Warning, slog.Any("settings", settings))
}

// workspaceDeprecationSources returns where the workspace settings that v3.0
// stops applying are read from. A missing store or service stays a nil
// source, which Find skips.
func workspaceDeprecationSources(cfg *config.Config, dr *persis.DAGRepository, stores Stores, authSvc *authservice.Service) deprecation.Sources {
	src := deprecation.Sources{
		DAGsDir:            cfg.Paths.DAGsDir,
		WikiDir:            cfg.Paths.WikiDir,
		ConfigKeys:         cfg.WorkspaceAccessKeys,
		Secrets:            stores.Secret,
		Profiles:           stores.Profile,
		NotificationRoutes: stores.Notification,
		IncidentPolicies:   stores.Incident,
	}
	if dr != nil {
		src.DAGs = dr
	}
	if authSvc != nil {
		src.Accounts = authSvc
	}
	return src
}
