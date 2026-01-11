package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/service/audit"
	authservice "github.com/dagu-org/dagu/internal/service/auth"
	"github.com/google/uuid"
	openapitypes "github.com/oapi-codegen/runtime/types"
)

const (
	// maxWebhookPayloadSize is the maximum size of the webhook payload in bytes (1MB).
	maxWebhookPayloadSize = 1 * 1024 * 1024
)

// ListWebhooks returns all webhooks across all DAGs.
// Requires admin role.
func (a *API) ListWebhooks(ctx context.Context, _ api.ListWebhooksRequestObject) (api.ListWebhooksResponseObject, error) {
	if err := a.requireWebhookManagement(ctx); err != nil {
		return nil, err
	}

	webhooks, err := a.authService.ListWebhooks(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to list webhooks", tag.Error(err))
		return nil, &Error{
			HTTPStatus: http.StatusInternalServerError,
			Code:       api.ErrorCodeInternalError,
			Message:    "failed to list webhooks",
		}
	}

	response := make([]api.WebhookDetails, 0, len(webhooks))
	for _, wh := range webhooks {
		response = append(response, toWebhookDetails(wh))
	}

	return api.ListWebhooks200JSONResponse{Webhooks: response}, nil
}

// GetDAGWebhook returns the webhook configuration for a specific DAG.
// Requires admin role.
func (a *API) GetDAGWebhook(ctx context.Context, request api.GetDAGWebhookRequestObject) (api.GetDAGWebhookResponseObject, error) {
	if err := a.requireWebhookManagement(ctx); err != nil {
		return nil, err
	}

	webhook, err := a.authService.GetWebhookByDAGName(ctx, request.FileName)
	if err != nil {
		if errors.Is(err, auth.ErrWebhookNotFound) {
			return nil, &Error{
				HTTPStatus: http.StatusNotFound,
				Code:       api.ErrorCodeNotFound,
				Message:    fmt.Sprintf("no webhook configured for DAG %s", request.FileName),
			}
		}
		logger.Error(ctx, "Failed to get webhook", tag.Name(request.FileName), tag.Error(err))
		return nil, &Error{
			HTTPStatus: http.StatusInternalServerError,
			Code:       api.ErrorCodeInternalError,
			Message:    "failed to get webhook",
		}
	}

	return api.GetDAGWebhook200JSONResponse(toWebhookDetails(webhook)), nil
}

// CreateDAGWebhook creates a new webhook for a DAG.
// Requires admin role.
func (a *API) CreateDAGWebhook(ctx context.Context, request api.CreateDAGWebhookRequestObject) (api.CreateDAGWebhookResponseObject, error) {
	if err := a.requireWebhookManagement(ctx); err != nil {
		return nil, err
	}

	// Check if DAG exists
	if _, err := a.dagStore.GetDetails(ctx, request.FileName); err != nil {
		if errors.Is(err, execution.ErrDAGNotFound) {
			return nil, &Error{
				HTTPStatus: http.StatusNotFound,
				Code:       api.ErrorCodeNotFound,
				Message:    fmt.Sprintf("DAG %s not found", request.FileName),
			}
		}
		return nil, err
	}

	// Get creator ID from context
	creatorID := getCreatorID(ctx)

	result, err := a.authService.CreateWebhook(ctx, request.FileName, creatorID)
	if err != nil {
		if errors.Is(err, auth.ErrWebhookAlreadyExists) {
			return nil, &Error{
				HTTPStatus: http.StatusConflict,
				Code:       api.ErrorCodeAlreadyExists,
				Message:    fmt.Sprintf("webhook already exists for DAG %s", request.FileName),
			}
		}
		logger.Error(ctx, "Failed to create webhook", tag.Name(request.FileName), tag.Error(err))
		return nil, &Error{
			HTTPStatus: http.StatusInternalServerError,
			Code:       api.ErrorCodeInternalError,
			Message:    "failed to create webhook",
		}
	}

	logger.Info(ctx, "Webhook created", tag.Name(request.FileName))

	// Log webhook creation
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		details, err := json.Marshal(map[string]string{
			"dag_name":   request.FileName,
			"webhook_id": result.Webhook.ID,
		})
		if err != nil {
			logger.Warn(ctx, "Failed to marshal audit details", tag.Error(err))
			details = []byte("{}")
		}
		entry := audit.NewEntry(audit.CategoryWebhook, "webhook_create", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
	}

	return api.CreateDAGWebhook201JSONResponse{
		Webhook: toWebhookDetails(result.Webhook),
		Token:   result.FullToken,
	}, nil
}

// DeleteDAGWebhook removes the webhook for a DAG.
// Requires admin role.
func (a *API) DeleteDAGWebhook(ctx context.Context, request api.DeleteDAGWebhookRequestObject) (api.DeleteDAGWebhookResponseObject, error) {
	if err := a.requireWebhookManagement(ctx); err != nil {
		return nil, err
	}

	// Get webhook info before deletion for audit logging
	targetWebhook, _ := a.authService.GetWebhookByDAGName(ctx, request.FileName)

	err := a.authService.DeleteWebhook(ctx, request.FileName)
	if err != nil {
		if errors.Is(err, auth.ErrWebhookNotFound) {
			return nil, &Error{
				HTTPStatus: http.StatusNotFound,
				Code:       api.ErrorCodeNotFound,
				Message:    fmt.Sprintf("no webhook configured for DAG %s", request.FileName),
			}
		}
		logger.Error(ctx, "Failed to delete webhook", tag.Name(request.FileName), tag.Error(err))
		return nil, &Error{
			HTTPStatus: http.StatusInternalServerError,
			Code:       api.ErrorCodeInternalError,
			Message:    "failed to delete webhook",
		}
	}

	logger.Info(ctx, "Webhook deleted", tag.Name(request.FileName))

	// Log webhook deletion
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		detailsMap := map[string]string{"dag_name": request.FileName}
		if targetWebhook != nil {
			detailsMap["webhook_id"] = targetWebhook.ID
		}
		details, err := json.Marshal(detailsMap)
		if err != nil {
			logger.Warn(ctx, "Failed to marshal audit details", tag.Error(err))
			details = []byte("{}")
		}
		entry := audit.NewEntry(audit.CategoryWebhook, "webhook_delete", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
	}

	return api.DeleteDAGWebhook204Response{}, nil
}

// RegenerateDAGWebhookToken generates a new token for an existing webhook.
// Requires admin role.
func (a *API) RegenerateDAGWebhookToken(ctx context.Context, request api.RegenerateDAGWebhookTokenRequestObject) (api.RegenerateDAGWebhookTokenResponseObject, error) {
	if err := a.requireWebhookManagement(ctx); err != nil {
		return nil, err
	}

	result, err := a.authService.RegenerateWebhookToken(ctx, request.FileName)
	if err != nil {
		if errors.Is(err, auth.ErrWebhookNotFound) {
			return nil, &Error{
				HTTPStatus: http.StatusNotFound,
				Code:       api.ErrorCodeNotFound,
				Message:    fmt.Sprintf("no webhook configured for DAG %s", request.FileName),
			}
		}
		logger.Error(ctx, "Failed to regenerate webhook token", tag.Name(request.FileName), tag.Error(err))
		return nil, &Error{
			HTTPStatus: http.StatusInternalServerError,
			Code:       api.ErrorCodeInternalError,
			Message:    "failed to regenerate webhook token",
		}
	}

	logger.Info(ctx, "Webhook token regenerated", tag.Name(request.FileName))

	// Log webhook token regeneration
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		details, err := json.Marshal(map[string]string{
			"dag_name":   request.FileName,
			"webhook_id": result.Webhook.ID,
		})
		if err != nil {
			logger.Warn(ctx, "Failed to marshal audit details", tag.Error(err))
			details = []byte("{}")
		}
		entry := audit.NewEntry(audit.CategoryWebhook, "webhook_token_regenerate", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
	}

	return api.RegenerateDAGWebhookToken200JSONResponse{
		Webhook: toWebhookDetails(result.Webhook),
		Token:   result.FullToken,
	}, nil
}

// ToggleDAGWebhook enables or disables a webhook.
// Requires admin role.
func (a *API) ToggleDAGWebhook(ctx context.Context, request api.ToggleDAGWebhookRequestObject) (api.ToggleDAGWebhookResponseObject, error) {
	if err := a.requireWebhookManagement(ctx); err != nil {
		return nil, err
	}

	webhook, err := a.authService.ToggleWebhook(ctx, request.FileName, request.Body.Enabled)
	if err != nil {
		if errors.Is(err, auth.ErrWebhookNotFound) {
			return nil, &Error{
				HTTPStatus: http.StatusNotFound,
				Code:       api.ErrorCodeNotFound,
				Message:    fmt.Sprintf("no webhook configured for DAG %s", request.FileName),
			}
		}
		logger.Error(ctx, "Failed to toggle webhook", tag.Name(request.FileName), tag.Error(err))
		return nil, &Error{
			HTTPStatus: http.StatusInternalServerError,
			Code:       api.ErrorCodeInternalError,
			Message:    "failed to toggle webhook",
		}
	}

	logger.Info(ctx, "Webhook toggled",
		tag.Name(request.FileName),
		tag.Key("enabled"), tag.Value(request.Body.Enabled),
	)

	// Log webhook toggle
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		details, err := json.Marshal(map[string]any{
			"dag_name":   request.FileName,
			"webhook_id": webhook.ID,
			"enabled":    request.Body.Enabled,
		})
		if err != nil {
			logger.Warn(ctx, "Failed to marshal audit details", tag.Error(err))
			details = []byte("{}")
		}
		entry := audit.NewEntry(audit.CategoryWebhook, "webhook_toggle", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
	}

	return api.ToggleDAGWebhook200JSONResponse(toWebhookDetails(webhook)), nil
}

// TriggerWebhook triggers a DAG execution via webhook.
// Authentication is performed using a bearer token specific to the webhook.
func (a *API) TriggerWebhook(ctx context.Context, request api.TriggerWebhookRequestObject) (api.TriggerWebhookResponseObject, error) {
	// Ensure webhook triggering is configured on this server
	if a.authService == nil || !a.authService.HasWebhookStore() {
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    "webhook triggering is not configured on this server",
		}
	}

	// Validate the token via auth service
	authHeader := ""
	if request.Params.Authorization != nil {
		authHeader = *request.Params.Authorization
	}
	token := extractWebhookToken(authHeader)
	if token == "" {
		logger.Warn(ctx, "Webhook: missing or invalid authorization header",
			tag.Name(request.FileName),
		)
		return nil, &Error{
			HTTPStatus: http.StatusUnauthorized,
			Code:       api.ErrorCodeUnauthorized,
			Message:    "missing or invalid authorization header",
		}
	}

	// Validate token and check if webhook is enabled
	webhook, err := a.authService.ValidateWebhookToken(ctx, request.FileName, token)
	if err != nil {
		if errors.Is(err, authservice.ErrInvalidWebhookToken) {
			logger.Warn(ctx, "Webhook: invalid token", tag.Name(request.FileName))
			return nil, &Error{
				HTTPStatus: http.StatusUnauthorized,
				Code:       api.ErrorCodeUnauthorized,
				Message:    "invalid webhook token",
			}
		}
		if errors.Is(err, authservice.ErrWebhookDisabled) {
			logger.Warn(ctx, "Webhook: disabled", tag.Name(request.FileName))
			return nil, &Error{
				HTTPStatus: http.StatusForbidden,
				Code:       api.ErrorCodeForbidden,
				Message:    "webhook is disabled",
			}
		}
		if errors.Is(err, authservice.ErrWebhookNotConfigured) {
			logger.Warn(ctx, "Webhook: not configured", tag.Name(request.FileName))
			return nil, &Error{
				HTTPStatus: http.StatusNotFound,
				Code:       api.ErrorCodeNotFound,
				Message:    "no webhook configured for this DAG",
			}
		}
		logger.Error(ctx, "Webhook: validation failed", tag.Name(request.FileName), tag.Error(err))
		return nil, &Error{
			HTTPStatus: http.StatusInternalServerError,
			Code:       api.ErrorCodeInternalError,
			Message:    "webhook validation failed",
		}
	}

	// Load the DAG (we need it for enqueuing)
	dag, err := a.dagStore.GetDetails(ctx, request.FileName)
	if err != nil {
		logger.Warn(ctx, "Webhook: DAG not found",
			tag.Name(request.FileName),
			tag.Error(err),
		)
		return nil, &Error{
			HTTPStatus: http.StatusNotFound,
			Code:       api.ErrorCodeNotFound,
			Message:    fmt.Sprintf("DAG %s not found", request.FileName),
		}
	}

	// Prepare the WEBHOOK_PAYLOAD parameter
	var payload string
	if request.Body != nil && request.Body.Payload != nil {
		payloadBytes, err := json.Marshal(*request.Body.Payload)
		if err != nil {
			logger.Warn(ctx, "Webhook: failed to marshal payload",
				tag.Name(dag.Name),
				tag.Error(err),
			)
			return nil, &Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    "failed to process request body",
			}
		}
		// Check payload size to prevent memory exhaustion attacks
		if len(payloadBytes) > maxWebhookPayloadSize {
			logger.Warn(ctx, "Webhook: payload too large",
				tag.Name(dag.Name),
				tag.Key("size"), tag.Value(len(payloadBytes)),
				tag.Key("maxSize"), tag.Value(maxWebhookPayloadSize),
			)
			return nil, &Error{
				HTTPStatus: http.StatusRequestEntityTooLarge,
				Code:       api.ErrorCodeBadRequest,
				Message:    fmt.Sprintf("webhook payload too large (max %d bytes)", maxWebhookPayloadSize),
			}
		}
		payload = string(payloadBytes)
	} else {
		payload = "{}"
	}

	// Create the params string with WEBHOOK_PAYLOAD
	// Use strconv.Quote to properly escape the JSON payload.
	// The parameter parser regex splits on whitespace for unquoted values,
	// so we need to quote the value to preserve spaces in JSON.
	params := fmt.Sprintf("WEBHOOK_PAYLOAD=%s", strconv.Quote(payload))

	// Determine the dag-run ID (use provided one for idempotency, or generate new)
	var dagRunID string
	if request.Body != nil && request.Body.DagRunId != nil && *request.Body.DagRunId != "" {
		dagRunID = *request.Body.DagRunId
		// Check if a dag-run with this ID already exists
		statuses, err := a.dagRunStore.ListStatuses(ctx, execution.WithDAGRunID(dagRunID))
		if err == nil && len(statuses) > 0 {
			// DAG run already exists - return 409 Conflict
			logger.Info(ctx, "Webhook: DAG run already exists (idempotency)",
				tag.Name(dag.Name),
				tag.Key("dagRunID"), tag.Value(dagRunID),
			)
			return api.TriggerWebhook409JSONResponse{
				Code:    api.ErrorCodeAlreadyExists,
				Message: fmt.Sprintf("dag-run with ID %s already exists", dagRunID),
			}, nil
		}
		// If no results or error, proceed with creating
	} else {
		dagRunID = uuid.Must(uuid.NewV7()).String()
	}

	// Enqueue the DAG run
	if err := a.enqueueDAGRun(ctx, dag, params, dagRunID, ""); err != nil {
		logger.Error(ctx, "Webhook: failed to enqueue DAG run",
			tag.Name(dag.Name),
			tag.Error(err),
		)
		return nil, &Error{
			HTTPStatus: http.StatusInternalServerError,
			Code:       api.ErrorCodeInternalError,
			Message:    "failed to enqueue DAG run",
		}
	}

	logger.Info(ctx, "Webhook: DAG run enqueued",
		tag.Name(dag.Name),
		tag.Key("dagRunID"), tag.Value(dagRunID),
		tag.Key("webhookID"), tag.Value(webhook.ID),
	)

	return api.TriggerWebhook200JSONResponse{
		DagRunId: dagRunID,
		DagName:  dag.Name,
	}, nil
}

// requireWebhookManagement checks if webhook management is enabled and
// enforces admin role for all webhook management operations.
func (a *API) requireWebhookManagement(ctx context.Context) error {
	if a.authService == nil || !a.authService.HasWebhookStore() {
		return &Error{
			Code:       api.ErrorCodeUnauthorized,
			Message:    "Webhook management is not enabled",
			HTTPStatus: http.StatusUnauthorized,
		}
	}
	// Enforce admin role when auth is enabled
	return a.requireAdmin(ctx)
}

// extractWebhookToken extracts the token from the Authorization header.
// It expects the format "Bearer <token>".
func extractWebhookToken(authHeader string) string {
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return ""
	}
	return strings.TrimPrefix(authHeader, bearerPrefix)
}

// toWebhookDetails converts an auth.Webhook to an api.WebhookDetails.
func toWebhookDetails(wh *auth.Webhook) api.WebhookDetails {
	// Parse UUID - if invalid (shouldn't happen as we generate it), use nil UUID
	parsedID, err := uuid.Parse(wh.ID)
	if err != nil {
		parsedID = uuid.Nil
	}

	details := api.WebhookDetails{
		Id:          openapitypes.UUID(parsedID),
		DagName:     wh.DAGName,
		TokenPrefix: wh.TokenPrefix,
		Enabled:     wh.Enabled,
		CreatedAt:   wh.CreatedAt,
		UpdatedAt:   wh.UpdatedAt,
		CreatedBy:   ptrOf(wh.CreatedBy),
	}
	if wh.LastUsedAt != nil {
		details.LastUsedAt = wh.LastUsedAt
	}
	return details
}

// getCreatorID extracts the user ID from context or returns a default value.
func getCreatorID(ctx context.Context) string {
	user, ok := auth.UserFromContext(ctx)
	if ok && user != nil {
		return user.ID
	}
	return "system"
}
