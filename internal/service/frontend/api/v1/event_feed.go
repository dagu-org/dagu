// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"log/slog"

	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/service/eventfeed"
)

func (a *API) recordRecentEvent(ctx context.Context, entry eventfeed.Entry) {
	if a == nil || a.eventFeedService == nil {
		return
	}
	if err := a.eventFeedService.Record(ctx, entry); err != nil {
		logger.Warn(ctx, "Failed to record recent event",
			slog.String("event_type", string(entry.Type)),
			tag.DAG(entry.DAGName),
			tag.RunID(entry.DAGRunID),
			tag.Error(err),
		)
	}
}

func recentEventActor(ctx context.Context) string {
	user, ok := auth.UserFromContext(ctx)
	if !ok || user == nil {
		return ""
	}
	return user.Username
}
