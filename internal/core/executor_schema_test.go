package core

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateExecutorConfig_ValidConfig(t *testing.T) {
	type testConfig struct {
		Name string `json:"name"`
	}

	RegisterExecutorConfigType[testConfig]("test_valid")

	err := ValidateExecutorConfig("test_valid", map[string]any{"name": "foo"})
	require.NoError(t, err)
}

func TestValidateExecutorConfig_InvalidType(t *testing.T) {
	type testConfig struct {
		Name string `json:"name"`
	}

	RegisterExecutorConfigType[testConfig]("test_invalid_type")

	// Name should be string, not int
	err := ValidateExecutorConfig("test_invalid_type", map[string]any{"name": 123})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid test_invalid_type config")
}

func TestValidateExecutorConfig_UnknownField(t *testing.T) {
	type testConfig struct {
		Name string `json:"name"`
	}

	RegisterExecutorConfigType[testConfig]("test_unknown_field")

	// Unknown field should be rejected (additionalProperties: false)
	err := ValidateExecutorConfig("test_unknown_field", map[string]any{"unknown": true})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid test_unknown_field config")
}

func TestValidateExecutorConfig_NoSchemaRegistered(t *testing.T) {
	// Unregistered executor should pass (backward compatible)
	err := ValidateExecutorConfig("unregistered_executor", map[string]any{"any": "thing"})
	require.NoError(t, err)
}

func TestValidateExecutorConfig_EmptyConfig(t *testing.T) {
	type testConfig struct {
		Optional string `json:"optional,omitempty"`
	}

	RegisterExecutorConfigType[testConfig]("test_empty")

	// Empty config should be valid for optional-only fields
	err := ValidateExecutorConfig("test_empty", map[string]any{})
	require.NoError(t, err)
}

func TestValidateExecutorConfig_ConcurrentAccess(t *testing.T) {
	type testConfig struct {
		Value int `json:"value"`
	}

	RegisterExecutorConfigType[testConfig]("test_concurrent")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = ValidateExecutorConfig("test_concurrent", map[string]any{"value": 42})
		}()
	}
	wg.Wait()
}
