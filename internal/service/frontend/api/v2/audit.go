package api

import (
	"context"
	"net/http"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/service/audit"
)

// ListAuditLogs returns audit log entries matching the filter criteria.
// Requires admin role.
func (a *API) ListAuditLogs(ctx context.Context, request api.ListAuditLogsRequestObject) (api.ListAuditLogsResponseObject, error) {
	// Check that audit service is configured
	if a.auditService == nil {
		return nil, &Error{
			Code:       api.ErrorCodeInternalError,
			Message:    "Audit logging is not configured",
			HTTPStatus: http.StatusServiceUnavailable,
		}
	}

	// Require admin role
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
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

// GetAuditService returns the audit service for use by other components (e.g., terminal handler).
func (a *API) GetAuditService() *audit.Service {
	return a.auditService
}

// AuditEntry is a helper to log terminal session events.
// This is used by the terminal handler.
func (a *API) LogAuditEntry(ctx context.Context, category audit.Category, action, userID, username, details, ipAddress string) error {
	if a.auditService == nil {
		return nil // Silently skip if audit not configured
	}
	entry := audit.NewEntry(category, action, userID, username)
	if details != "" {
		entry.WithDetails(details)
	}
	if ipAddress != "" {
		entry.WithIPAddress(ipAddress)
	}
	return a.auditService.Log(ctx, entry)
}
