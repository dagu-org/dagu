// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"errors"
	"time"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/core/auth"
	authservice "github.com/dagu-org/dagu/internal/service/auth"
)

// Login authenticates a user and returns a JWT token.
func (a *API) Login(ctx context.Context, request api.LoginRequestObject) (api.LoginResponseObject, error) {
	if a.authService == nil {
		return api.Login401JSONResponse{
			Code:    api.ErrorCodeUnauthorized,
			Message: "Authentication is not enabled",
		}, nil
	}

	if request.Body == nil {
		return api.Login401JSONResponse{
			Code:    api.ErrorCodeUnauthorized,
			Message: "Invalid request body",
		}, nil
	}

	user, err := a.authService.Authenticate(ctx, request.Body.Username, request.Body.Password)
	if err != nil {
		if errors.Is(err, authservice.ErrInvalidCredentials) {
			return api.Login401JSONResponse{
				Code:    api.ErrorCodeUnauthorized,
				Message: "Invalid username or password",
			}, nil
		}
		return nil, err
	}

	token, err := a.authService.GenerateToken(user)
	if err != nil {
		return nil, err
	}

	claims, err := a.authService.ValidateToken(token)
	if err != nil {
		return nil, err
	}

	return api.Login200JSONResponse{
		Token:     token,
		ExpiresAt: claims.ExpiresAt.Time,
		User:      toAPIUser(user),
	}, nil
}

// GetCurrentUser returns the currently authenticated user.
func (a *API) GetCurrentUser(ctx context.Context, _ api.GetCurrentUserRequestObject) (api.GetCurrentUserResponseObject, error) {
	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return api.GetCurrentUser401JSONResponse{
			Code:    api.ErrorCodeUnauthorized,
			Message: "Not authenticated",
		}, nil
	}

	return api.GetCurrentUser200JSONResponse{
		User: toAPIUser(user),
	}, nil
}

// ChangePassword allows the authenticated user to change their own password.
func (a *API) ChangePassword(ctx context.Context, request api.ChangePasswordRequestObject) (api.ChangePasswordResponseObject, error) {
	if a.authService == nil {
		return api.ChangePassword401JSONResponse{
			Code:    api.ErrorCodeUnauthorized,
			Message: "Authentication is not enabled",
		}, nil
	}

	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return api.ChangePassword401JSONResponse{
			Code:    api.ErrorCodeUnauthorized,
			Message: "Not authenticated",
		}, nil
	}

	if request.Body == nil {
		return api.ChangePassword400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: "Invalid request body",
		}, nil
	}

	err := a.authService.ChangePassword(ctx, user.ID, request.Body.CurrentPassword, request.Body.NewPassword)
	if err != nil {
		if errors.Is(err, authservice.ErrPasswordMismatch) {
			return api.ChangePassword401JSONResponse{
				Code:    api.ErrorCodeUnauthorized,
				Message: "Current password is incorrect",
			}, nil
		}
		if errors.Is(err, authservice.ErrWeakPassword) {
			return api.ChangePassword400JSONResponse{
				Code:    api.ErrorCodeBadRequest,
				Message: "New password does not meet security requirements",
			}, nil
		}
		return nil, err
	}

	return api.ChangePassword200JSONResponse{
		Message: "Password changed successfully",
	}, nil
}

// toAPIUser converts a core auth.User to an API User.
func toAPIUser(user *auth.User) api.User {
	return api.User{
		Id:        user.ID,
		Username:  user.Username,
		Role:      api.UserRole(user.Role),
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

// toAPIUsers converts a slice of core auth.User to API Users.
func toAPIUsers(users []*auth.User) []api.User {
	result := make([]api.User, len(users))
	for i, u := range users {
		result[i] = toAPIUser(u)
	}
	return result
}

// formatTime formats a time for display.
func formatTime(t time.Time) string {
	return t.Format(time.RFC3339)
}
