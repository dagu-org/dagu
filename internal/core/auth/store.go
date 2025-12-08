// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package auth

import (
	"context"
	"errors"
)

// Common errors for user store operations.
var (
	// ErrUserNotFound is returned when a user cannot be found.
	ErrUserNotFound = errors.New("user not found")
	// ErrUserAlreadyExists is returned when attempting to create a user
	// with a username that already exists.
	ErrUserAlreadyExists = errors.New("user already exists")
	// ErrInvalidUsername is returned when the username is invalid.
	ErrInvalidUsername = errors.New("invalid username")
	// ErrInvalidUserID is returned when the user ID is invalid.
	ErrInvalidUserID = errors.New("invalid user ID")
)

// UserStore defines the interface for user persistence operations.
// Implementations must be safe for concurrent use.
type UserStore interface {
	// Create stores a new user.
	// Returns ErrUserAlreadyExists if a user with the same username exists.
	Create(ctx context.Context, user *User) error

	// GetByID retrieves a user by their unique ID.
	// Returns ErrUserNotFound if the user does not exist.
	GetByID(ctx context.Context, id string) (*User, error)

	// GetByUsername retrieves a user by their username.
	// Returns ErrUserNotFound if the user does not exist.
	GetByUsername(ctx context.Context, username string) (*User, error)

	// List returns all users in the store.
	List(ctx context.Context) ([]*User, error)

	// Update modifies an existing user.
	// Returns ErrUserNotFound if the user does not exist.
	Update(ctx context.Context, user *User) error

	// Delete removes a user by their ID.
	// Returns ErrUserNotFound if the user does not exist.
	Delete(ctx context.Context, id string) error

	// Count returns the total number of users.
	Count(ctx context.Context) (int64, error)
}
