// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/auth"
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
