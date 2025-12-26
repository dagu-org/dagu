// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package auth

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAPIKey(t *testing.T) {
	before := time.Now().UTC()
	key := NewAPIKey("test-key", "Test description", RoleManager, "hash123", "dagu_tes", "creator-id")
	after := time.Now().UTC()

	assert.NotEmpty(t, key.ID, "ID should be generated")
	assert.Equal(t, "test-key", key.Name)
	assert.Equal(t, "Test description", key.Description)
	assert.Equal(t, RoleManager, key.Role)
	assert.Equal(t, "hash123", key.KeyHash)
	assert.Equal(t, "dagu_tes", key.KeyPrefix)
	assert.Equal(t, "creator-id", key.CreatedBy)
	assert.Nil(t, key.LastUsedAt)

	// CreatedAt and UpdatedAt should be set to approximately now
	assert.True(t, !key.CreatedAt.Before(before), "CreatedAt should be >= before")
	assert.True(t, !key.CreatedAt.After(after), "CreatedAt should be <= after")
	assert.Equal(t, key.CreatedAt, key.UpdatedAt, "CreatedAt and UpdatedAt should be equal initially")
}

func TestAPIKey_ToStorage(t *testing.T) {
	now := time.Now().UTC()
	lastUsed := now.Add(-time.Hour)
	key := &APIKey{
		ID:          "key-id",
		Name:        "test-key",
		Description: "Test description",
		Role:        RoleAdmin,
		KeyHash:     "hash123",
		KeyPrefix:   "dagu_tes",
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   "creator-id",
		LastUsedAt:  &lastUsed,
	}

	storage := key.ToStorage()

	assert.Equal(t, key.ID, storage.ID)
	assert.Equal(t, key.Name, storage.Name)
	assert.Equal(t, key.Description, storage.Description)
	assert.Equal(t, key.Role, storage.Role)
	assert.Equal(t, key.KeyHash, storage.KeyHash)
	assert.Equal(t, key.KeyPrefix, storage.KeyPrefix)
	assert.Equal(t, key.CreatedAt, storage.CreatedAt)
	assert.Equal(t, key.UpdatedAt, storage.UpdatedAt)
	assert.Equal(t, key.CreatedBy, storage.CreatedBy)
	require.NotNil(t, storage.LastUsedAt)
	assert.Equal(t, *key.LastUsedAt, *storage.LastUsedAt)
}

func TestAPIKeyForStorage_ToAPIKey(t *testing.T) {
	now := time.Now().UTC()
	lastUsed := now.Add(-time.Hour)
	storage := &APIKeyForStorage{
		ID:          "key-id",
		Name:        "test-key",
		Description: "Test description",
		Role:        RoleViewer,
		KeyHash:     "hash456",
		KeyPrefix:   "dagu_xyz",
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   "admin-user",
		LastUsedAt:  &lastUsed,
	}

	key := storage.ToAPIKey()

	assert.Equal(t, storage.ID, key.ID)
	assert.Equal(t, storage.Name, key.Name)
	assert.Equal(t, storage.Description, key.Description)
	assert.Equal(t, storage.Role, key.Role)
	assert.Equal(t, storage.KeyHash, key.KeyHash)
	assert.Equal(t, storage.KeyPrefix, key.KeyPrefix)
	assert.Equal(t, storage.CreatedAt, key.CreatedAt)
	assert.Equal(t, storage.UpdatedAt, key.UpdatedAt)
	assert.Equal(t, storage.CreatedBy, key.CreatedBy)
	require.NotNil(t, key.LastUsedAt)
	assert.Equal(t, *storage.LastUsedAt, *key.LastUsedAt)
}

func TestAPIKey_ToStorage_ToAPIKey_Roundtrip(t *testing.T) {
	now := time.Now().UTC()
	original := &APIKey{
		ID:          "key-id",
		Name:        "roundtrip-key",
		Description: "Roundtrip test",
		Role:        RoleOperator,
		KeyHash:     "secret-hash",
		KeyPrefix:   "dagu_rnd",
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   "creator",
	}

	// Convert to storage and back
	storage := original.ToStorage()
	recovered := storage.ToAPIKey()

	assert.Equal(t, original.ID, recovered.ID)
	assert.Equal(t, original.Name, recovered.Name)
	assert.Equal(t, original.Description, recovered.Description)
	assert.Equal(t, original.Role, recovered.Role)
	assert.Equal(t, original.KeyHash, recovered.KeyHash)
	assert.Equal(t, original.KeyPrefix, recovered.KeyPrefix)
	assert.Equal(t, original.CreatedAt, recovered.CreatedAt)
	assert.Equal(t, original.UpdatedAt, recovered.UpdatedAt)
	assert.Equal(t, original.CreatedBy, recovered.CreatedBy)
	assert.Equal(t, original.LastUsedAt, recovered.LastUsedAt)
}

func TestAPIKey_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second) // Truncate for JSON round-trip
	key := &APIKey{
		ID:          "key-id",
		Name:        "json-key",
		Description: "JSON test",
		Role:        RoleAdmin,
		KeyHash:     "should-be-excluded",
		KeyPrefix:   "dagu_jsn",
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   "creator",
	}

	// Serialize to JSON
	data, err := json.Marshal(key)
	require.NoError(t, err)

	// KeyHash should NOT be in the JSON (json:"-" tag)
	jsonStr := string(data)
	assert.NotContains(t, jsonStr, "should-be-excluded")
	assert.NotContains(t, jsonStr, "key_hash")

	// Deserialize back
	var recovered APIKey
	err = json.Unmarshal(data, &recovered)
	require.NoError(t, err)

	assert.Equal(t, key.ID, recovered.ID)
	assert.Equal(t, key.Name, recovered.Name)
	assert.Equal(t, key.Description, recovered.Description)
	assert.Equal(t, key.Role, recovered.Role)
	assert.Equal(t, key.KeyPrefix, recovered.KeyPrefix)
	assert.Empty(t, recovered.KeyHash, "KeyHash should not be deserialized")
}

func TestAPIKeyForStorage_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second) // Truncate for JSON round-trip
	storage := &APIKeyForStorage{
		ID:          "key-id",
		Name:        "storage-key",
		Description: "Storage test",
		Role:        RoleManager,
		KeyHash:     "included-hash",
		KeyPrefix:   "dagu_str",
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   "admin",
	}

	// Serialize to JSON
	data, err := json.Marshal(storage)
	require.NoError(t, err)

	// KeyHash SHOULD be in the JSON for storage
	jsonStr := string(data)
	assert.Contains(t, jsonStr, "included-hash")
	assert.Contains(t, jsonStr, "key_hash")

	// Deserialize back
	var recovered APIKeyForStorage
	err = json.Unmarshal(data, &recovered)
	require.NoError(t, err)

	assert.Equal(t, storage.ID, recovered.ID)
	assert.Equal(t, storage.Name, recovered.Name)
	assert.Equal(t, storage.Role, recovered.Role)
	assert.Equal(t, storage.KeyHash, recovered.KeyHash)
}

func TestAPIKey_NilLastUsedAt(t *testing.T) {
	key := &APIKey{
		ID:        "key-id",
		Name:      "nil-lastused",
		Role:      RoleViewer,
		KeyHash:   "hash",
		KeyPrefix: "dagu_nil",
		CreatedBy: "creator",
	}

	// LastUsedAt is nil by default
	assert.Nil(t, key.LastUsedAt)

	// Convert to storage and back
	storage := key.ToStorage()
	assert.Nil(t, storage.LastUsedAt)

	recovered := storage.ToAPIKey()
	assert.Nil(t, recovered.LastUsedAt)
}

func TestNewAPIKey_GeneratesUniqueIDs(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key := NewAPIKey("test", "", RoleViewer, "hash", "prefix", "creator")
		assert.False(t, ids[key.ID], "ID should be unique")
		ids[key.ID] = true
	}
}
