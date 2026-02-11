package agent

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSlugID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "typical model name", input: "Claude Opus 4.6", want: "claude-opus-4-6"},
		{name: "empty string", input: "", want: ""},
		{name: "leading and trailing spaces", input: "  spaces  ", want: "spaces"},
		{name: "special characters", input: "gpt-4@turbo!v2", want: "gpt-4-turbo-v2"},
		{name: "already a slug", input: "my-model", want: "my-model"},
		{name: "uppercase", input: "ABC", want: "abc"},
		{name: "multiple consecutive specials", input: "a---b", want: "a-b"},
		{name: "only special chars", input: "!@#$%", want: ""},
		{name: "mixed whitespace", input: "hello\tworld\nnew", want: "hello-world-new"},
		{name: "numbers only", input: "12345", want: "12345"},
		{name: "unicode characters", input: "modele-francais", want: "modele-francais"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := GenerateSlugID(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestUniqueID_NoCollision(t *testing.T) {
	t.Parallel()

	existing := map[string]struct{}{}
	id := UniqueID("Claude Opus 4.6", existing)
	assert.Equal(t, "claude-opus-4-6", id)
}

func TestUniqueID_Collision(t *testing.T) {
	t.Parallel()

	existing := map[string]struct{}{
		"claude-opus-4-6": {},
	}
	id := UniqueID("Claude Opus 4.6", existing)
	assert.Equal(t, "claude-opus-4-6-2", id)
}

func TestUniqueID_MultipleCollisions(t *testing.T) {
	t.Parallel()

	existing := map[string]struct{}{
		"claude-opus-4-6":   {},
		"claude-opus-4-6-2": {},
		"claude-opus-4-6-3": {},
	}
	id := UniqueID("Claude Opus 4.6", existing)
	assert.Equal(t, "claude-opus-4-6-4", id)
}

func TestUniqueID_EmptyNameUsesModel(t *testing.T) {
	t.Parallel()

	existing := map[string]struct{}{}
	id := UniqueID("", existing)
	assert.Equal(t, "model", id)
}

func TestUniqueID_EmptyNameCollision(t *testing.T) {
	t.Parallel()

	existing := map[string]struct{}{
		"model": {},
	}
	id := UniqueID("", existing)
	assert.Equal(t, "model-2", id)
}

func TestValidateModelID(t *testing.T) {
	t.Parallel()

	t.Run("valid IDs", func(t *testing.T) {
		t.Parallel()

		validIDs := []string{
			"claude-opus-4",
			"gpt-4-1-mini",
			"a",
			"abc123",
			"model-1-2-3",
			"a-b",
		}

		for _, id := range validIDs {
			t.Run(id, func(t *testing.T) {
				t.Parallel()
				err := ValidateModelID(id)
				assert.NoError(t, err, "expected %q to be valid", id)
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
			{name: "too long", id: strings.Repeat("a", 129)},
			{name: "path traversal", id: "../../etc/passwd"},
			{name: "uppercase", id: "ABC"},
			{name: "spaces", id: "has spaces"},
			{name: "dots", id: "model.v1"},
			{name: "leading hyphen", id: "-leading"},
			{name: "trailing hyphen", id: "trailing-"},
			{name: "consecutive hyphens", id: "a--b"},
			{name: "slash", id: "a/b"},
			{name: "underscore", id: "a_b"},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				err := ValidateModelID(tc.id)
				require.Error(t, err, "expected %q to be invalid", tc.id)
				assert.ErrorIs(t, err, ErrInvalidModelID)
			})
		}
	})

	t.Run("boundary length 128 is valid", func(t *testing.T) {
		t.Parallel()
		id := strings.Repeat("a", 128)
		err := ValidateModelID(id)
		assert.NoError(t, err, "128-char ID should be valid")
	})
}
