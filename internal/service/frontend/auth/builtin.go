// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dagu-org/dagu/internal/core/auth"
)

// TokenValidator defines the interface for validating tokens and retrieving users.
// This allows the middleware to work with any token validation implementation.
type TokenValidator interface {
	GetUserFromToken(ctx context.Context, token string) (*auth.User, error)
}

// BuiltinAuthMiddleware creates middleware that validates JWT tokens
// and injects the authenticated user into the request context.
// Public paths are excluded from authentication.
func BuiltinAuthMiddleware(svc TokenValidator, publicPaths []string) func(http.Handler) http.Handler {
	// Build a set for O(1) lookup
	publicSet := make(map[string]struct{}, len(publicPaths))
	for _, p := range publicPaths {
		publicSet[p] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path is public
			if isPublicPath(r.URL.Path, publicSet) {
				next.ServeHTTP(w, r)
				return
			}

			token := extractBearerToken(r)
			if token == "" {
				writeAuthError(w, http.StatusUnauthorized, "auth.unauthorized", "Authentication required")
				return
			}

			user, err := svc.GetUserFromToken(r.Context(), token)
			if err != nil {
				writeAuthError(w, http.StatusUnauthorized, "auth.token_invalid", "Invalid or expired token")
				return
			}

			// Inject user into context
			ctx := auth.WithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// isPublicPath checks if the request path matches any public path.
// Handles trailing slash normalization bidirectionally.
func isPublicPath(path string, publicSet map[string]struct{}) bool {
	// Exact match
	if _, ok := publicSet[path]; ok {
		return true
	}
	// Try with trailing slash removed
	withoutSlash := strings.TrimSuffix(path, "/")
	if _, ok := publicSet[withoutSlash]; ok {
		return true
	}
	// Try with trailing slash added
	if path != "" && !strings.HasSuffix(path, "/") {
		if _, ok := publicSet[path+"/"]; ok {
			return true
		}
	}
	return false
}

// RequireRole creates middleware that checks if the authenticated user
// has one of the required roles.
func RequireRole(roles ...auth.Role) func(http.Handler) http.Handler {
	roleSet := make(map[auth.Role]struct{}, len(roles))
	for _, r := range roles {
		roleSet[r] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := auth.UserFromContext(r.Context())
			if !ok {
				writeAuthError(w, http.StatusUnauthorized, "auth.unauthorized", "Authentication required")
				return
			}

			if _, allowed := roleSet[user.Role]; !allowed {
				writeAuthError(w, http.StatusForbidden, "auth.forbidden", "Insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin is a convenience middleware that requires admin role.
func RequireAdmin() func(http.Handler) http.Handler {
	return RequireRole(auth.RoleAdmin)
}

// RequireWrite is a convenience middleware that requires write permissions (admin or manager).
func RequireWrite() func(http.Handler) http.Handler {
	return RequireRole(auth.RoleAdmin, auth.RoleManager)
}

// RequireExecute is a convenience middleware that requires execute permissions (admin, manager, or operator).
func RequireExecute() func(http.Handler) http.Handler {
	return RequireRole(auth.RoleAdmin, auth.RoleManager, auth.RoleOperator)
}

// extractBearerToken extracts the JWT token from the Authorization header.
func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return ""
	}

	return strings.TrimPrefix(authHeader, bearerPrefix)
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error details.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeAuthError writes a JSON error response.
func writeAuthError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
	})
}
