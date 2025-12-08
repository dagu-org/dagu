// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package auth

import "context"

// contextKey is a private type for context keys to avoid collisions.
type contextKey string

const (
	// userContextKey is the key for storing the authenticated user in context.
	userContextKey contextKey = "auth_user"
)

// WithUser returns a new context with the user attached.
func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// UserFromContext retrieves the authenticated user from the context.
// Returns the user and true if found, nil and false otherwise.
func UserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(userContextKey).(*User)
	return user, ok
}

// MustUserFromContext retrieves the authenticated user from the context.
// Panics if no user is found. Use only after RequireAuth middleware.
func MustUserFromContext(ctx context.Context) *User {
	user, ok := UserFromContext(ctx)
	if !ok {
		panic("auth: no user in context (did you forget RequireAuth middleware?)")
	}
	return user
}
