package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	t.Run("returns correct defaults", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultConfig()

		assert.True(t, cfg.Enabled)
		assert.Equal(t, DefaultProvider, cfg.LLM.Provider)
		assert.Equal(t, DefaultModel, cfg.LLM.Model)
	})

	t.Run("returns new instance each time", func(t *testing.T) {
		t.Parallel()

		cfg1 := DefaultConfig()
		cfg2 := DefaultConfig()

		// Modify cfg1
		cfg1.Enabled = false
		cfg1.LLM.Model = "modified"

		// cfg2 should still have defaults
		assert.True(t, cfg2.Enabled)
		assert.Equal(t, DefaultModel, cfg2.LLM.Model)
	})
}

func TestErrorConstants(t *testing.T) {
	t.Parallel()

	t.Run("ErrConversationNotFound has descriptive message", func(t *testing.T) {
		t.Parallel()

		assert.NotNil(t, ErrConversationNotFound)
		assert.Contains(t, ErrConversationNotFound.Error(), "not found")
	})

	t.Run("ErrInvalidConversationID has descriptive message", func(t *testing.T) {
		t.Parallel()

		assert.NotNil(t, ErrInvalidConversationID)
		assert.Contains(t, ErrInvalidConversationID.Error(), "invalid")
	})

	t.Run("ErrInvalidUserID has descriptive message", func(t *testing.T) {
		t.Parallel()

		assert.NotNil(t, ErrInvalidUserID)
		assert.Contains(t, ErrInvalidUserID.Error(), "invalid")
	})
}
