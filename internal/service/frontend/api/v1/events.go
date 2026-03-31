// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"maps"
	"net/http"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/service/eventstore"
)

// ListEventLogs returns centralized event log entries matching the filter criteria.
// Requires manager or admin role.
func (a *API) ListEventLogs(ctx context.Context, request api.ListEventLogsRequestObject) (api.ListEventLogsResponseObject, error) {
	if err := a.requireManagerOrAbove(ctx); err != nil {
		return nil, err
	}

	if a.eventService == nil {
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Event logging is not configured",
			HTTPStatus: http.StatusServiceUnavailable,
		}
	}

	filter := eventstore.QueryFilter{}
	if request.Params.Kind != nil {
		filter.Kind = eventstore.EventKind(*request.Params.Kind)
	}
	if request.Params.Type != nil {
		filter.Type = eventstore.EventType(*request.Params.Type)
	}
	if request.Params.DagName != nil {
		filter.DAGName = *request.Params.DagName
	}
	if request.Params.DagRunId != nil {
		filter.DAGRunID = *request.Params.DagRunId
	}
	if request.Params.AttemptId != nil {
		filter.AttemptID = *request.Params.AttemptId
	}
	if request.Params.SessionId != nil {
		filter.SessionID = *request.Params.SessionId
	}
	if request.Params.UserId != nil {
		filter.UserID = *request.Params.UserId
	}
	if request.Params.Model != nil {
		filter.Model = *request.Params.Model
	}
	if request.Params.StartTime != nil {
		filter.StartTime = *request.Params.StartTime
	}
	if request.Params.EndTime != nil {
		filter.EndTime = *request.Params.EndTime
	}
	if request.Params.Limit != nil {
		filter.Limit = *request.Params.Limit
	}
	if request.Params.Offset != nil {
		filter.Offset = *request.Params.Offset
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

	result, err := a.eventService.Query(ctx, filter)
	if err != nil {
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Failed to query event logs",
			HTTPStatus: http.StatusInternalServerError,
		}
	}
	if result == nil {
		result = &eventstore.QueryResult{}
	}

	entries := make([]api.EventLogEntry, 0, len(result.Entries))
	for _, e := range result.Entries {
		if e == nil {
			continue
		}
		entry := api.EventLogEntry{
			Id:            e.ID,
			SchemaVersion: e.SchemaVersion,
			OccurredAt:    e.OccurredAt,
			RecordedAt:    e.RecordedAt,
			Kind:          string(e.Kind),
			Type:          string(e.Type),
			SourceService: e.SourceService,
		}
		if e.SourceInstance != "" {
			entry.SourceInstance = &e.SourceInstance
		}
		if e.DAGName != "" {
			entry.DagName = &e.DAGName
		}
		if e.DAGRunID != "" {
			entry.DagRunId = &e.DAGRunID
		}
		if e.AttemptID != "" {
			entry.AttemptId = &e.AttemptID
		}
		if e.SessionID != "" {
			entry.SessionId = &e.SessionID
		}
		if e.UserID != "" {
			entry.UserId = &e.UserID
		}
		if e.Model != "" {
			entry.Model = &e.Model
		}
		if e.Status != "" {
			entry.Status = &e.Status
		}
		if len(e.Data) > 0 {
			data := make(map[string]any, len(e.Data))
			maps.Copy(data, e.Data)
			entry.Data = &data
		}
		entries = append(entries, entry)
	}

	return api.ListEventLogs200JSONResponse{
		Entries: entries,
		Total:   result.Total,
	}, nil
}
