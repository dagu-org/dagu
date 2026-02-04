package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/service/audit"
	authservice "github.com/dagu-org/dagu/internal/service/auth"
)

// ListAPIKeys returns a list of all API keys. Requires admin role.
func (a *API) ListAPIKeys(ctx context.Context, _ api.ListAPIKeysRequestObject) (api.ListAPIKeysResponseObject, error) {
	if err := a.requireAPIKeyManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	keys, err := a.authService.ListAPIKeys(ctx)
	if err != nil {
		return nil, err
	}

	return api.ListAPIKeys200JSONResponse{
		ApiKeys: toAPIKeys(keys),
	}, nil
}

// CreateAPIKey creates a new API key. Requires admin role.
func (a *API) CreateAPIKey(ctx context.Context, request api.CreateAPIKeyRequestObject) (api.CreateAPIKeyResponseObject, error) {
	if err := a.requireAPIKeyManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if request.Body == nil {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    "Invalid request body",
			HTTPStatus: http.StatusBadRequest,
		}
	}

	role, err := auth.ParseRole(string(request.Body.Role))
	if err != nil {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    "Invalid role",
			HTTPStatus: http.StatusBadRequest,
		}
	}

	// Get current user for createdBy
	currentUser, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, &Error{
			Code:       api.ErrorCodeUnauthorized,
			Message:    "Not authenticated",
			HTTPStatus: http.StatusUnauthorized,
		}
	}

	result, err := a.authService.CreateAPIKey(ctx, authservice.CreateAPIKeyInput{
		Name:        request.Body.Name,
		Description: valueOf(request.Body.Description),
		Role:        role,
	}, currentUser.ID)
	if err != nil {
		if errors.Is(err, auth.ErrAPIKeyAlreadyExists) {
			return nil, &Error{
				Code:       api.ErrorCodeAlreadyExists,
				Message:    "API key with this name already exists",
				HTTPStatus: http.StatusConflict,
			}
		}
		if errors.Is(err, auth.ErrInvalidAPIKeyName) {
			return nil, &Error{
				Code:       api.ErrorCodeBadRequest,
				Message:    "Invalid API key name",
				HTTPStatus: http.StatusBadRequest,
			}
		}
		return nil, err
	}

	// Log API key creation
	if a.auditService != nil {
		clientIP, _ := auth.ClientIPFromContext(ctx)
		details, err := json.Marshal(map[string]string{
			"key_id":   result.APIKey.ID,
			"key_name": result.APIKey.Name,
			"role":     string(result.APIKey.Role),
		})
		if err != nil {
			logger.Warn(ctx, "Failed to marshal audit details", tag.Error(err))
			details = []byte("{}")
		}
		entry := audit.NewEntry(audit.CategoryAPIKey, "api_key_create", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
	}

	return api.CreateAPIKey201JSONResponse{
		ApiKey: toAPIKey(result.APIKey),
		Key:    result.FullKey,
	}, nil
}

// GetAPIKey returns a specific API key by ID. Requires admin role.
func (a *API) GetAPIKey(ctx context.Context, request api.GetAPIKeyRequestObject) (api.GetAPIKeyResponseObject, error) {
	if err := a.requireAPIKeyManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	key, err := a.authService.GetAPIKey(ctx, request.KeyId)
	if err != nil {
		if errors.Is(err, auth.ErrAPIKeyNotFound) {
			return nil, &Error{
				Code:       api.ErrorCodeNotFound,
				Message:    "API key not found",
				HTTPStatus: http.StatusNotFound,
			}
		}
		return nil, err
	}

	return api.GetAPIKey200JSONResponse{
		ApiKey: toAPIKey(key),
	}, nil
}

// UpdateAPIKey updates an API key's information. Requires admin role.
func (a *API) UpdateAPIKey(ctx context.Context, request api.UpdateAPIKeyRequestObject) (api.UpdateAPIKeyResponseObject, error) {
	if err := a.requireAPIKeyManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if request.Body == nil {
		return nil, &Error{
			Code:       api.ErrorCodeBadRequest,
			Message:    "Invalid request body",
			HTTPStatus: http.StatusBadRequest,
		}
	}

	input := authservice.UpdateAPIKeyInput{}
	if request.Body.Name != nil {
		input.Name = request.Body.Name
	}
	if request.Body.Description != nil {
		input.Description = request.Body.Description
	}
	if request.Body.Role != nil {
		role, err := auth.ParseRole(string(*request.Body.Role))
		if err != nil {
			return nil, &Error{
				Code:       api.ErrorCodeBadRequest,
				Message:    "Invalid role",
				HTTPStatus: http.StatusBadRequest,
			}
		}
		input.Role = &role
	}

	key, err := a.authService.UpdateAPIKey(ctx, request.KeyId, input)
	if err != nil {
		if errors.Is(err, auth.ErrAPIKeyNotFound) {
			return nil, &Error{
				Code:       api.ErrorCodeNotFound,
				Message:    "API key not found",
				HTTPStatus: http.StatusNotFound,
			}
		}
		if errors.Is(err, auth.ErrAPIKeyAlreadyExists) {
			return nil, &Error{
				Code:       api.ErrorCodeAlreadyExists,
				Message:    "API key with this name already exists",
				HTTPStatus: http.StatusConflict,
			}
		}
		if errors.Is(err, auth.ErrInvalidAPIKeyName) {
			return nil, &Error{
				Code:       api.ErrorCodeBadRequest,
				Message:    "Invalid API key name",
				HTTPStatus: http.StatusBadRequest,
			}
		}
		return nil, err
	}

	// Log API key update
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		changes := make(map[string]any)
		changes["key_id"] = request.KeyId
		if input.Name != nil {
			changes["name"] = *input.Name
		}
		if input.Description != nil {
			changes["description"] = *input.Description
		}
		if input.Role != nil {
			changes["role"] = string(*input.Role)
		}
		details, err := json.Marshal(changes)
		if err != nil {
			logger.Warn(ctx, "Failed to marshal audit details", tag.Error(err))
			details = []byte("{}")
		}
		entry := audit.NewEntry(audit.CategoryAPIKey, "api_key_update", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
	}

	return api.UpdateAPIKey200JSONResponse{
		ApiKey: toAPIKey(key),
	}, nil
}

// DeleteAPIKey deletes an API key. Requires admin role.
func (a *API) DeleteAPIKey(ctx context.Context, request api.DeleteAPIKeyRequestObject) (api.DeleteAPIKeyResponseObject, error) {
	if err := a.requireAPIKeyManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	// Get API key info before deletion for audit logging
	targetKey, _ := a.authService.GetAPIKey(ctx, request.KeyId)

	err := a.authService.DeleteAPIKey(ctx, request.KeyId)
	if err != nil {
		if errors.Is(err, auth.ErrAPIKeyNotFound) {
			return nil, &Error{
				Code:       api.ErrorCodeNotFound,
				Message:    "API key not found",
				HTTPStatus: http.StatusNotFound,
			}
		}
		return nil, err
	}

	// Log API key deletion
	if a.auditService != nil {
		currentUser, _ := auth.UserFromContext(ctx)
		clientIP, _ := auth.ClientIPFromContext(ctx)
		detailsMap := map[string]string{"key_id": request.KeyId}
		if targetKey != nil {
			detailsMap["key_name"] = targetKey.Name
		}
		details, err := json.Marshal(detailsMap)
		if err != nil {
			logger.Warn(ctx, "Failed to marshal audit details", tag.Error(err))
			details = []byte("{}")
		}
		entry := audit.NewEntry(audit.CategoryAPIKey, "api_key_delete", currentUser.ID, currentUser.Username).
			WithDetails(string(details)).
			WithIPAddress(clientIP)
		_ = a.auditService.Log(ctx, entry)
	}

	return api.DeleteAPIKey204Response{}, nil
}

// requireAPIKeyManagement checks if API key management is enabled.
func (a *API) requireAPIKeyManagement() error {
	if a.authService == nil || !a.authService.HasAPIKeyStore() {
		return &Error{
			Code:       api.ErrorCodeForbidden,
			Message:    "API key management is not available",
			HTTPStatus: http.StatusForbidden,
		}
	}
	return nil
}

// toAPIKey converts a core auth.APIKey into its API representation.
func toAPIKey(key *auth.APIKey) api.APIKey {
	return api.APIKey{
		Id:          key.ID,
		Name:        key.Name,
		Description: ptrOf(key.Description),
		Role:        api.UserRole(key.Role),
		KeyPrefix:   key.KeyPrefix,
		CreatedAt:   key.CreatedAt,
		UpdatedAt:   key.UpdatedAt,
		CreatedBy:   key.CreatedBy,
		LastUsedAt:  key.LastUsedAt,
	}
}

// toAPIKeys converts a slice of core auth.APIKey into their API representations.
func toAPIKeys(keys []*auth.APIKey) []api.APIKey {
	result := make([]api.APIKey, len(keys))
	for i, k := range keys {
		result[i] = toAPIKey(k)
	}
	return result
}
