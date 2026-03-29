// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"net/http"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/service/eventfeed"
)

// ListRecentEvents returns recent workflow lifecycle events.
func (a *API) ListRecentEvents(ctx context.Context, request api.ListRecentEventsRequestObject) (api.ListRecentEventsResponseObject, error) {
	if a.eventFeedService == nil {
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Recent event feed is not configured",
			HTTPStatus: http.StatusServiceUnavailable,
		}
	}

	filter := eventfeed.QueryFilter{
		DAGName:   valueOf(request.Params.DagName),
		DAGRunID:  valueOf(request.Params.DagRunId),
		Actor:     valueOf(request.Params.Actor),
		Search:    valueOf(request.Params.Search),
		StartTime: valueOf(request.Params.StartTime),
		EndTime:   valueOf(request.Params.EndTime),
		Limit:     valueOf(request.Params.Limit),
		Offset:    valueOf(request.Params.Offset),
	}
	if request.Params.Type != nil {
		filter.Type = eventfeed.EventType(*request.Params.Type)
	}

	const (
		defaultLimit = 50
		maxLimit     = 500
	)
	if filter.Limit <= 0 {
		filter.Limit = defaultLimit
	} else if filter.Limit > maxLimit {
		filter.Limit = maxLimit
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	result, err := a.eventFeedService.Query(ctx, filter)
	if err != nil {
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Failed to query recent events",
			HTTPStatus: http.StatusInternalServerError,
		}
	}

	entries := make([]api.RecentEventEntry, 0, len(result.Entries))
	for _, e := range result.Entries {
		entry := api.RecentEventEntry{
			Id:        e.ID,
			Timestamp: e.Timestamp,
			Type:      api.RecentEventType(e.Type),
			DagName:   e.DAGName,
			DagRunId:  e.DAGRunID,
		}
		if e.SubDAGRunID != "" {
			entry.SubDAGRunId = &e.SubDAGRunID
		}
		if e.AttemptID != "" {
			entry.AttemptId = &e.AttemptID
		}
		if e.StepName != "" {
			entry.StepName = &e.StepName
		}
		if e.Actor != "" {
			entry.Actor = &e.Actor
		}
		if e.Reason != "" {
			entry.Reason = &e.Reason
		}
		if e.ApprovalIteration != nil {
			entry.ApprovalIteration = e.ApprovalIteration
		}
		if e.Resumed != nil {
			entry.Resumed = e.Resumed
		}
		if e.ResultingRunStatus != "" {
			entry.ResultingRunStatus = &e.ResultingRunStatus
		}
		entries = append(entries, entry)
	}

	return api.ListRecentEvents200JSONResponse{
		Entries: entries,
		Total:   result.Total,
	}, nil
}
