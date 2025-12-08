// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package auth

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system.
type User struct {
	// ID is the unique identifier for the user (UUID).
	ID string `json:"id"`
	// Username is the unique login name.
	Username string `json:"username"`
	// PasswordHash is the bcrypt hash of the password.
	// Excluded from JSON serialization for security.
	PasswordHash string `json:"-"`
	// Role determines the user's permissions.
	Role Role `json:"role"`
	// CreatedAt is the timestamp when the user was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the user was last modified.
	UpdatedAt time.Time `json:"updated_at"`
}

// NewUser creates a new User with a generated UUID and timestamps.
func NewUser(username string, passwordHash string, role Role) *User {
	now := time.Now().UTC()
	return &User{
		ID:           uuid.New().String(),
		Username:     username,
		PasswordHash: passwordHash,
		Role:         role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// UserForStorage is used for JSON serialization to persistent storage.
// It includes the password hash which is excluded from the regular User JSON.
type UserForStorage struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ToStorage converts a User to UserForStorage for persistence.
func (u *User) ToStorage() *UserForStorage {
	return &UserForStorage{
		ID:           u.ID,
		Username:     u.Username,
		PasswordHash: u.PasswordHash,
		Role:         u.Role,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

// ToUser converts UserForStorage back to User.
func (s *UserForStorage) ToUser() *User {
	return &User{
		ID:           s.ID,
		Username:     s.Username,
		PasswordHash: s.PasswordHash,
		Role:         s.Role,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
	}
}
