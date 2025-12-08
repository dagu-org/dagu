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

// WithUser returns a new context that carries the provided user value.
func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// UserFromContext retrieves the authenticated user from the context.
// UserFromContext retrieves the authenticated *User stored in ctx.
// It returns the user and true if a *User value is present for the package's userContextKey, or nil and false otherwise.
func UserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(userContextKey).(*User)
	return user, ok
}