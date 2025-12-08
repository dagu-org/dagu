// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"errors"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/core/auth"
	authservice "github.com/dagu-org/dagu/internal/service/auth"
)

// ListUsers returns a list of all users. Requires admin role.
func (a *API) ListUsers(ctx context.Context, _ api.ListUsersRequestObject) (api.ListUsersResponseObject, error) {
	if a.authService == nil {
		return api.ListUsers401JSONResponse{
			Code:    api.ErrorCodeUnauthorized,
			Message: "User management is not enabled",
		}, nil
	}

	if err := a.requireAdmin(ctx); err != nil {
		return api.ListUsers403JSONResponse{
			Code:    api.ErrorCodeForbidden,
			Message: err.Error(),
		}, nil
	}

	users, err := a.authService.ListUsers(ctx)
	if err != nil {
		return nil, err
	}

	return api.ListUsers200JSONResponse{
		Users: toAPIUsers(users),
	}, nil
}

// CreateUser creates a new user. Requires admin role.
func (a *API) CreateUser(ctx context.Context, request api.CreateUserRequestObject) (api.CreateUserResponseObject, error) {
	if a.authService == nil {
		return api.CreateUser401JSONResponse{
			Code:    api.ErrorCodeUnauthorized,
			Message: "User management is not enabled",
		}, nil
	}

	if err := a.requireAdmin(ctx); err != nil {
		return api.CreateUser403JSONResponse{
			Code:    api.ErrorCodeForbidden,
			Message: err.Error(),
		}, nil
	}

	if request.Body == nil {
		return api.CreateUser400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: "Invalid request body",
		}, nil
	}

	role, err := auth.ParseRole(string(request.Body.Role))
	if err != nil {
		return api.CreateUser400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: "Invalid role",
		}, nil
	}

	user, err := a.authService.CreateUser(ctx, authservice.CreateUserInput{
		Username: request.Body.Username,
		Password: request.Body.Password,
		Role:     role,
	})
	if err != nil {
		if errors.Is(err, auth.ErrUserAlreadyExists) {
			return api.CreateUser409JSONResponse{
				Code:    api.ErrorCodeAlreadyExists,
				Message: "Username already exists",
			}, nil
		}
		if errors.Is(err, authservice.ErrWeakPassword) {
			return api.CreateUser400JSONResponse{
				Code:    api.ErrorCodeBadRequest,
				Message: "Password does not meet security requirements",
			}, nil
		}
		return nil, err
	}

	return api.CreateUser201JSONResponse{
		User: toAPIUser(user),
	}, nil
}

// GetUser returns a specific user by ID. Requires admin role.
func (a *API) GetUser(ctx context.Context, request api.GetUserRequestObject) (api.GetUserResponseObject, error) {
	if a.authService == nil {
		return api.GetUser401JSONResponse{
			Code:    api.ErrorCodeUnauthorized,
			Message: "User management is not enabled",
		}, nil
	}

	if err := a.requireAdmin(ctx); err != nil {
		return api.GetUser403JSONResponse{
			Code:    api.ErrorCodeForbidden,
			Message: err.Error(),
		}, nil
	}

	user, err := a.authService.GetUser(ctx, request.UserId)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return api.GetUser404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: "User not found",
			}, nil
		}
		return nil, err
	}

	return api.GetUser200JSONResponse{
		User: toAPIUser(user),
	}, nil
}

// UpdateUser updates a user's information. Requires admin role.
func (a *API) UpdateUser(ctx context.Context, request api.UpdateUserRequestObject) (api.UpdateUserResponseObject, error) {
	if a.authService == nil {
		return api.UpdateUser401JSONResponse{
			Code:    api.ErrorCodeUnauthorized,
			Message: "User management is not enabled",
		}, nil
	}

	if err := a.requireAdmin(ctx); err != nil {
		return api.UpdateUser403JSONResponse{
			Code:    api.ErrorCodeForbidden,
			Message: err.Error(),
		}, nil
	}

	if request.Body == nil {
		return api.UpdateUser400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: "Invalid request body",
		}, nil
	}

	input := authservice.UpdateUserInput{}
	if request.Body.Username != nil {
		input.Username = request.Body.Username
	}
	if request.Body.Role != nil {
		role, err := auth.ParseRole(string(*request.Body.Role))
		if err != nil {
			return api.UpdateUser400JSONResponse{
				Code:    api.ErrorCodeBadRequest,
				Message: "Invalid role",
			}, nil
		}
		input.Role = &role
	}

	user, err := a.authService.UpdateUser(ctx, request.UserId, input)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return api.UpdateUser404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: "User not found",
			}, nil
		}
		if errors.Is(err, auth.ErrUserAlreadyExists) {
			return api.UpdateUser409JSONResponse{
				Code:    api.ErrorCodeAlreadyExists,
				Message: "Username already exists",
			}, nil
		}
		return nil, err
	}

	return api.UpdateUser200JSONResponse{
		User: toAPIUser(user),
	}, nil
}

// DeleteUser deletes a user account. Requires admin role. Cannot delete yourself.
func (a *API) DeleteUser(ctx context.Context, request api.DeleteUserRequestObject) (api.DeleteUserResponseObject, error) {
	if a.authService == nil {
		return api.DeleteUser401JSONResponse{
			Code:    api.ErrorCodeUnauthorized,
			Message: "User management is not enabled",
		}, nil
	}

	if err := a.requireAdmin(ctx); err != nil {
		return api.DeleteUser403JSONResponse{
			Code:    api.ErrorCodeForbidden,
			Message: err.Error(),
		}, nil
	}

	// Get current user to prevent self-deletion
	currentUser, ok := auth.UserFromContext(ctx)
	if !ok {
		return api.DeleteUser401JSONResponse{
			Code:    api.ErrorCodeUnauthorized,
			Message: "Not authenticated",
		}, nil
	}

	err := a.authService.DeleteUser(ctx, request.UserId, currentUser.ID)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return api.DeleteUser404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: "User not found",
			}, nil
		}
		if errors.Is(err, authservice.ErrCannotDeleteSelf) {
			return api.DeleteUser403JSONResponse{
				Code:    api.ErrorCodeForbidden,
				Message: "Cannot delete your own account",
			}, nil
		}
		return nil, err
	}

	return api.DeleteUser204Response{}, nil
}

// ResetUserPassword resets a user's password. Requires admin role.
func (a *API) ResetUserPassword(ctx context.Context, request api.ResetUserPasswordRequestObject) (api.ResetUserPasswordResponseObject, error) {
	if a.authService == nil {
		return api.ResetUserPassword401JSONResponse{
			Code:    api.ErrorCodeUnauthorized,
			Message: "User management is not enabled",
		}, nil
	}

	if err := a.requireAdmin(ctx); err != nil {
		return api.ResetUserPassword403JSONResponse{
			Code:    api.ErrorCodeForbidden,
			Message: err.Error(),
		}, nil
	}

	if request.Body == nil {
		return api.ResetUserPassword400JSONResponse{
			Code:    api.ErrorCodeBadRequest,
			Message: "Invalid request body",
		}, nil
	}

	err := a.authService.ResetPassword(ctx, request.UserId, request.Body.NewPassword)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return api.ResetUserPassword404JSONResponse{
				Code:    api.ErrorCodeNotFound,
				Message: "User not found",
			}, nil
		}
		if errors.Is(err, authservice.ErrWeakPassword) {
			return api.ResetUserPassword400JSONResponse{
				Code:    api.ErrorCodeBadRequest,
				Message: "Password does not meet security requirements",
			}, nil
		}
		return nil, err
	}

	return api.ResetUserPassword200JSONResponse{
		Message: "Password reset successfully",
	}, nil
}
