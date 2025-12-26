// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package auth

import (
	"time"

	"github.com/google/uuid"
)

// APIKey represents a standalone API key in the system.
// API keys are independent entities with their own role assignment,
// enabling programmatic access with fine-grained permissions.
type APIKey struct {
	// ID is the unique identifier for the API key (UUID).
	ID string `json:"id"`
	// Name is a human-readable name for the API key (required).
	Name string `json:"name"`
	// Description is an optional description of the API key's purpose.
	Description string `json:"description,omitempty"`
	// Role determines the API key's permissions.
	Role Role `json:"role"`
	// KeyHash is the bcrypt hash of the API key secret.
	// Excluded from JSON serialization for security.
	KeyHash string `json:"-"`
	// KeyPrefix stores the first 8 characters of the key for identification.
	KeyPrefix string `json:"key_prefix"`
	// CreatedAt is the timestamp when the API key was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the API key was last modified.
	UpdatedAt time.Time `json:"updated_at"`
	// CreatedBy is the user ID of the admin who created the API key.
	CreatedBy string `json:"created_by"`
	// LastUsedAt is the timestamp when the API key was last used for authentication.
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// NewAPIKey creates an APIKey with a new UUID and sets CreatedAt and UpdatedAt to the current UTC time.
func NewAPIKey(name, description string, role Role, keyHash, keyPrefix, createdBy string) *APIKey {
	now := time.Now().UTC()
	return &APIKey{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Role:        role,
		KeyHash:     keyHash,
		KeyPrefix:   keyPrefix,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   createdBy,
	}
}

// APIKeyForStorage is used for JSON serialization to persistent storage.
// It includes the key hash which is excluded from the regular APIKey JSON.
type APIKeyForStorage struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Role        Role       `json:"role"`
	KeyHash     string     `json:"key_hash"`
	KeyPrefix   string     `json:"key_prefix"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CreatedBy   string     `json:"created_by"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
}

// ToStorage converts an APIKey to APIKeyForStorage for persistence.
func (k *APIKey) ToStorage() *APIKeyForStorage {
	return &APIKeyForStorage{
		ID:          k.ID,
		Name:        k.Name,
		Description: k.Description,
		Role:        k.Role,
		KeyHash:     k.KeyHash,
		KeyPrefix:   k.KeyPrefix,
		CreatedAt:   k.CreatedAt,
		UpdatedAt:   k.UpdatedAt,
		CreatedBy:   k.CreatedBy,
		LastUsedAt:  k.LastUsedAt,
	}
}

// ToAPIKey converts APIKeyForStorage back to APIKey.
func (s *APIKeyForStorage) ToAPIKey() *APIKey {
	return &APIKey{
		ID:          s.ID,
		Name:        s.Name,
		Description: s.Description,
		Role:        s.Role,
		KeyHash:     s.KeyHash,
		KeyPrefix:   s.KeyPrefix,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
		CreatedBy:   s.CreatedBy,
		LastUsedAt:  s.LastUsedAt,
	}
}
