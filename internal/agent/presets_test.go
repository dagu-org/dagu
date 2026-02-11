package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetModelPresets(t *testing.T) {
	t.Parallel()

	t.Run("returns non-empty slice", func(t *testing.T) {
		t.Parallel()

		presets := GetModelPresets()
		require.NotEmpty(t, presets)
	})

	t.Run("all presets have required fields", func(t *testing.T) {
		t.Parallel()

		presets := GetModelPresets()
		for _, p := range presets {
			assert.NotEmpty(t, p.Name, "preset Name must not be empty")
			assert.NotEmpty(t, p.Provider, "preset Provider must not be empty for %s", p.Name)
			assert.NotEmpty(t, p.Model, "preset Model must not be empty for %s", p.Name)
		}
	})

	t.Run("all presets have positive pricing", func(t *testing.T) {
		t.Parallel()

		presets := GetModelPresets()
		for _, p := range presets {
			assert.Greater(t, p.InputCostPer1M, 0.0, "InputCostPer1M should be > 0 for %s", p.Name)
			assert.Greater(t, p.OutputCostPer1M, 0.0, "OutputCostPer1M should be > 0 for %s", p.Name)
		}
	})

	t.Run("all presets have valid providers", func(t *testing.T) {
		t.Parallel()

		validProviders := map[string]bool{
			"anthropic":  true,
			"openai":     true,
			"gemini":     true,
			"openrouter": true,
			"local":      true,
		}

		presets := GetModelPresets()
		for _, p := range presets {
			assert.True(t, validProviders[p.Provider], "provider %q is not valid for preset %s", p.Provider, p.Name)
		}
	})
}

func TestGetModelPresets_ReturnsCopy(t *testing.T) {
	t.Parallel()

	t.Run("modifying returned slice does not affect subsequent calls", func(t *testing.T) {
		t.Parallel()

		first := GetModelPresets()
		originalName := first[0].Name

		// Mutate the returned slice
		first[0].Name = "MODIFIED"

		second := GetModelPresets()
		assert.Equal(t, originalName, second[0].Name, "second call should return unmodified data")
	})

	t.Run("returned slices are independent", func(t *testing.T) {
		t.Parallel()

		first := GetModelPresets()
		second := GetModelPresets()

		assert.Equal(t, len(first), len(second))

		// Append to first should not affect second
		first = append(first, ModelConfig{Name: "extra"})
		assert.NotEqual(t, len(first), len(second))
	})
}
