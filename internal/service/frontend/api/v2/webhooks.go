package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/google/uuid"
)

const (
	// maxWebhookPayloadSize is the maximum size of the webhook payload in bytes (1MB).
	maxWebhookPayloadSize = 1 * 1024 * 1024
	// maxWebhookFileNameLength is the maximum length for a webhook file name.
	maxWebhookFileNameLength = 255
)

// webhookFileNameRegex validates that the file name contains only safe characters.
// Allowed: alphanumeric, dash, underscore, dot (but not starting with dot).
var webhookFileNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// TriggerWebhook triggers a DAG execution via webhook.
// The DAG must have webhook enabled in its configuration.
// Authentication is performed using a bearer token specific to the DAG's webhook configuration.
func (a *API) TriggerWebhook(ctx context.Context, request api.TriggerWebhookRequestObject) (api.TriggerWebhookResponseObject, error) {
	// Validate the file name to prevent path traversal and injection attacks
	if err := ValidateWebhookFileName(request.FileName); err != nil {
		logger.Warn(ctx, "Webhook: invalid file name",
			tag.Name(request.FileName),
			tag.Error(err),
		)
		return nil, &Error{
			HTTPStatus: http.StatusBadRequest,
			Code:       api.ErrorCodeBadRequest,
			Message:    err.Error(),
		}
	}

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
	params := fmt.Sprintf("WEBHOOK_PAYLOAD=%s", payload)

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
	)

	return api.TriggerWebhook200JSONResponse{
		DagRunId: dagRunID,
	}, nil
}

// ValidateWebhookFileName validates the file name to prevent security issues.
// It checks for path traversal attempts, excessively long names, and invalid characters.
func ValidateWebhookFileName(fileName string) error {
	if fileName == "" {
		return fmt.Errorf("file name is required")
	}
	if len(fileName) > maxWebhookFileNameLength {
		return fmt.Errorf("file name too long (max %d characters)", maxWebhookFileNameLength)
	}
	// Check for path traversal attempts
	if strings.Contains(fileName, "..") || strings.Contains(fileName, "/") || strings.Contains(fileName, "\\") {
		return fmt.Errorf("invalid file name: path traversal not allowed")
	}
	// Validate characters
	if !webhookFileNameRegex.MatchString(fileName) {
		return fmt.Errorf("invalid file name: must start with alphanumeric and contain only alphanumeric, dash, underscore, or dot")
	}
	return nil
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
