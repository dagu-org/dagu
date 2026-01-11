package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/auth"
	authservice "github.com/dagu-org/dagu/internal/service/auth"
)

// ListUsers returns a list of all users. Requires admin role.
func (a *API) ListUsers(ctx context.Context, _ api.ListUsersRequestObject) (api.ListUsersResponseObject, error) {
	if err := a.requireUserManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
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
	if err := a.requireUserManagement(); err != nil {
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

	user, err := a.authService.CreateUser(ctx, authservice.CreateUserInput{
		Username: request.Body.Username,
		Password: request.Body.Password,
		Role:     role,
	})
	if err != nil {
		if errors.Is(err, auth.ErrUserAlreadyExists) {
			return nil, &Error{
				Code:       api.ErrorCodeAlreadyExists,
				Message:    "Username already exists",
				HTTPStatus: http.StatusConflict,
			}
		}
		if errors.Is(err, auth.ErrInvalidUsername) {
			return nil, &Error{
				Code:       api.ErrorCodeBadRequest,
				Message:    "Invalid username",
				HTTPStatus: http.StatusBadRequest,
			}
		}
		if errors.Is(err, authservice.ErrWeakPassword) {
			return nil, &Error{
				Code:       api.ErrorCodeBadRequest,
				Message:    "Password does not meet security requirements",
				HTTPStatus: http.StatusBadRequest,
			}
		}
		return nil, err
	}

	return api.CreateUser201JSONResponse{
		User: toAPIUser(user),
	}, nil
}

// GetUser returns a specific user by ID. Requires admin role.
func (a *API) GetUser(ctx context.Context, request api.GetUserRequestObject) (api.GetUserResponseObject, error) {
	if err := a.requireUserManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	user, err := a.authService.GetUser(ctx, request.UserId)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return nil, &Error{
				Code:       api.ErrorCodeNotFound,
				Message:    "User not found",
				HTTPStatus: http.StatusNotFound,
			}
		}
		return nil, err
	}

	return api.GetUser200JSONResponse{
		User: toAPIUser(user),
	}, nil
}

// UpdateUser updates a user's information. Requires admin role.
func (a *API) UpdateUser(ctx context.Context, request api.UpdateUserRequestObject) (api.UpdateUserResponseObject, error) {
	if err := a.requireUserManagement(); err != nil {
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

	input := authservice.UpdateUserInput{}
	if request.Body.Username != nil {
		input.Username = request.Body.Username
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
	if request.Body.IsDisabled != nil {
		input.IsDisabled = request.Body.IsDisabled
	}

	user, err := a.authService.UpdateUser(ctx, request.UserId, input)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return nil, &Error{
				Code:       api.ErrorCodeNotFound,
				Message:    "User not found",
				HTTPStatus: http.StatusNotFound,
			}
		}
		if errors.Is(err, auth.ErrUserAlreadyExists) {
			return nil, &Error{
				Code:       api.ErrorCodeAlreadyExists,
				Message:    "Username already exists",
				HTTPStatus: http.StatusConflict,
			}
		}
		if errors.Is(err, auth.ErrInvalidUsername) {
			return nil, &Error{
				Code:       api.ErrorCodeBadRequest,
				Message:    "Invalid username",
				HTTPStatus: http.StatusBadRequest,
			}
		}
		return nil, err
	}

	return api.UpdateUser200JSONResponse{
		User: toAPIUser(user),
	}, nil
}

// DeleteUser deletes a user account. Requires admin role. Cannot delete yourself.
func (a *API) DeleteUser(ctx context.Context, request api.DeleteUserRequestObject) (api.DeleteUserResponseObject, error) {
	if err := a.requireUserManagement(); err != nil {
		return nil, err
	}
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}

	// Get current user to prevent self-deletion
	currentUser, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, &Error{
			Code:       api.ErrorCodeUnauthorized,
			Message:    "Not authenticated",
			HTTPStatus: http.StatusUnauthorized,
		}
	}

	err := a.authService.DeleteUser(ctx, request.UserId, currentUser.ID)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return nil, &Error{
				Code:       api.ErrorCodeNotFound,
				Message:    "User not found",
				HTTPStatus: http.StatusNotFound,
			}
		}
		if errors.Is(err, authservice.ErrCannotDeleteSelf) {
			return nil, &Error{
				Code:       api.ErrorCodeForbidden,
				Message:    "Cannot delete your own account",
				HTTPStatus: http.StatusForbidden,
			}
		}
		return nil, err
	}

	return api.DeleteUser204Response{}, nil
}

// ResetUserPassword resets a user's password. Requires admin role.
func (a *API) ResetUserPassword(ctx context.Context, request api.ResetUserPasswordRequestObject) (api.ResetUserPasswordResponseObject, error) {
	if err := a.requireUserManagement(); err != nil {
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

	err := a.authService.ResetPassword(ctx, request.UserId, request.Body.NewPassword)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return nil, &Error{
				Code:       api.ErrorCodeNotFound,
				Message:    "User not found",
				HTTPStatus: http.StatusNotFound,
			}
		}
		if errors.Is(err, authservice.ErrWeakPassword) {
			return nil, &Error{
				Code:       api.ErrorCodeBadRequest,
				Message:    "Password does not meet security requirements",
				HTTPStatus: http.StatusBadRequest,
			}
		}
		return nil, err
	}

	return api.ResetUserPassword200JSONResponse{
		Message: "Password reset successfully",
	}, nil
}
