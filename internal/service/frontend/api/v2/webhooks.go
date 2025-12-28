package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/google/uuid"
)

// TriggerWebhook triggers a DAG execution via webhook.
// The DAG must have webhook enabled in its configuration.
// Authentication is performed using a bearer token specific to the DAG's webhook configuration.
func (a *API) TriggerWebhook(ctx context.Context, request api.TriggerWebhookRequestObject) (api.TriggerWebhookResponseObject, error) {
	// Load the DAG
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

	// Check if webhook is enabled
	if dag.Webhook == nil || !dag.Webhook.Enabled {
		logger.Warn(ctx, "Webhook: not enabled for DAG",
			tag.Name(dag.Name),
		)
		return nil, &Error{
			HTTPStatus: http.StatusForbidden,
			Code:       api.ErrorCodeForbidden,
			Message:    "webhook not enabled for this DAG",
		}
	}

	// Validate the token
	token := extractWebhookToken(request.Params.Authorization)
	if token == "" {
		logger.Warn(ctx, "Webhook: missing or invalid authorization header",
			tag.Name(dag.Name),
		)
		return nil, &Error{
			HTTPStatus: http.StatusUnauthorized,
			Code:       api.ErrorCodeUnauthorized,
			Message:    "missing or invalid authorization header",
		}
	}

	// Use constant-time comparison to prevent timing attacks
	if !validateWebhookToken(token, dag.Webhook.Token) {
		logger.Warn(ctx, "Webhook: invalid token",
			tag.Name(dag.Name),
		)
		return nil, &Error{
			HTTPStatus: http.StatusUnauthorized,
			Code:       api.ErrorCodeUnauthorized,
			Message:    "invalid webhook token",
		}
	}

	// Prepare the WEBHOOK_PAYLOAD parameter
	var payload string
	if request.Body != nil {
		payloadBytes, err := json.Marshal(*request.Body)
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
		payload = string(payloadBytes)
	} else {
		payload = "{}"
	}

	// Create the params string with WEBHOOK_PAYLOAD
	params := fmt.Sprintf("WEBHOOK_PAYLOAD=%s", payload)

	// Generate a dag-run ID
	dagRunID := uuid.Must(uuid.NewV7()).String()

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
	)

	return api.TriggerWebhook200JSONResponse{
		DagRunId: dagRunID,
	}, nil
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

// validateWebhookToken validates the token using constant-time comparison.
func validateWebhookToken(provided, expected string) bool {
	if provided == "" || expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}
