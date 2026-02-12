package api

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/service/audit"
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

	clientIP, _ := auth.ClientIPFromContext(ctx)

	user, err := a.authService.Authenticate(ctx, request.Body.Username, request.Body.Password)
	if err != nil {
		if errors.Is(err, authservice.ErrInvalidCredentials) {
			// Log failed login attempt
			if a.auditService != nil {
				details, err := json.Marshal(map[string]string{"reason": "invalid_credentials"})
				if err != nil {
					logger.Warn(ctx, "Failed to marshal audit details", tag.Error(err))
					details = []byte("{}")
				}
				entry := audit.NewEntry(audit.CategoryUser, "login_failed", "", request.Body.Username).
					WithDetails(string(details)).
					WithIPAddress(clientIP)
				_ = a.auditService.Log(ctx, entry)
			}
			return api.Login401JSONResponse{
				Code:    api.ErrorCodeUnauthorized,
				Message: "Invalid username or password",
			}, nil
		}
		return nil, err
	}

	tokenResult, err := a.authService.GenerateToken(user)
	if err != nil {
		return nil, err
	}

	a.logAudit(ctx, audit.CategoryUser, "login", nil)

	return api.Login200JSONResponse{
		Token:     tokenResult.Token,
		ExpiresAt: tokenResult.ExpiresAt,
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

	a.logAudit(ctx, audit.CategoryUser, "password_change", nil)

	return api.ChangePassword200JSONResponse{
		Message: "Password changed successfully",
	}, nil
}

// toAPIUser converts a core auth.User into its API representation.
// The provided user must be non-nil.
func toAPIUser(user *auth.User) api.User {
	// Default to "builtin" if auth provider is empty
	authProvider := cmp.Or(user.AuthProvider, "builtin")

	apiUser := api.User{
		Id:           user.ID,
		Username:     user.Username,
		Role:         api.UserRole(user.Role),
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		AuthProvider: (*api.UserAuthProvider)(&authProvider),
	}

	if user.IsDisabled {
		apiUser.IsDisabled = &user.IsDisabled
	}

	return apiUser
}

// preserving the input order.
func toAPIUsers(users []*auth.User) []api.User {
	result := make([]api.User, len(users))
	for i, u := range users {
		result[i] = toAPIUser(u)
	}
	return result
}
