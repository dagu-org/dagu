package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected Role
	}{
		{"system", RoleSystem},
		{"sys", RoleSystem},
		{"user", RoleUser},
		{"human", RoleUser},
		{"assistant", RoleAssistant},
		{"ai", RoleAssistant},
		{"bot", RoleAssistant},
		{"tool", RoleTool},
		{"function", RoleTool},
		{"custom", Role("custom")},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, ParseRole(tc.input))
		})
	}
}
