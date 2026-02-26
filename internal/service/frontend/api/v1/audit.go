package api

import (
	"context"
	"net/http"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/service/audit"
)

// ListAuditLogs returns audit log entries matching the filter criteria.
// Requires audit license and manager or admin role.
func (a *API) ListAuditLogs(ctx context.Context, request api.ListAuditLogsRequestObject) (api.ListAuditLogsResponseObject, error) {
	// Require manager or admin role (auth before license check)
	if err := a.requireManagerOrAbove(ctx); err != nil {
		return nil, err
	}

	if err := a.requireLicensedAudit(); err != nil {
		return nil, err
	}

	// Check that audit service is configured
	if a.auditService == nil {
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Audit logging is not configured",
			HTTPStatus: http.StatusServiceUnavailable,
		}
	}

	// Build filter from query parameters
	filter := audit.QueryFilter{}

	if request.Params.Category != nil {
		filter.Category = audit.Category(*request.Params.Category)
	}
	if request.Params.UserId != nil {
		filter.UserID = *request.Params.UserId
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

	// Apply pagination defaults and caps
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

	// Query the audit service
	result, err := a.auditService.Query(ctx, filter)
	if err != nil {
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Failed to query audit logs",
			HTTPStatus: http.StatusInternalServerError,
		}
	}

	// Convert to API response
	entries := make([]api.AuditEntry, 0, len(result.Entries))
	for _, e := range result.Entries {
		entry := api.AuditEntry{
			Id:        e.ID,
			Timestamp: e.Timestamp,
			Category:  string(e.Category),
			Action:    e.Action,
			UserId:    e.UserID,
			Username:  e.Username,
		}
		if e.Details != "" {
			entry.Details = &e.Details
		}
		if e.IPAddress != "" {
			entry.IpAddress = &e.IPAddress
		}
		entries = append(entries, entry)
	}

	return api.ListAuditLogs200JSONResponse{
		Entries: entries,
		Total:   result.Total,
	}, nil
}
