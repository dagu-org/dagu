package agent

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSkillID(t *testing.T) {
	t.Parallel()

	t.Run("valid IDs", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			id   string
		}{
			{name: "simple slug", id: "kubernetes"},
			{name: "multi-segment", id: "my-custom-skill"},
			{name: "single char", id: "a"},
			{name: "numeric", id: "123"},
			{name: "mixed", id: "skill-v2"},
			{name: "max length", id: strings.Repeat("a", 128)},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				err := ValidateSkillID(tt.id)
				require.NoError(t, err)
			})
		}
	})

	t.Run("invalid IDs", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			id   string
		}{
			{name: "empty", id: ""},
			{name: "uppercase", id: "BadId"},
			{name: "underscore", id: "bad_id"},
			{name: "path traversal", id: "../../etc"},
			{name: "slash prefix", id: "/etc/passwd"},
			{name: "space", id: "bad id"},
			{name: "leading hyphen", id: "-bad"},
			{name: "trailing hyphen", id: "bad-"},
			{name: "double hyphen", id: "bad--id"},
			{name: "dot", id: "bad.id"},
			{name: "dot-dot", id: ".."},
			{name: "single dot", id: "."},
			{name: "too long", id: strings.Repeat("a", 129)},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				err := ValidateSkillID(tt.id)
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidSkillID)
			})
		}
	})
}
