package execution

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeduplicateSystemMessages(t *testing.T) {
	tests := []struct {
		name string
		msgs []LLMMessage
		want []LLMMessage
	}{
		{
			name: "empty",
			msgs: []LLMMessage{},
			want: nil,
		},
		{
			name: "nil",
			msgs: nil,
			want: nil,
		},
		{
			name: "no system messages",
			msgs: []LLMMessage{
				{Role: RoleUser, Content: "hello"},
				{Role: RoleAssistant, Content: "hi"},
			},
			want: []LLMMessage{
				{Role: RoleUser, Content: "hello"},
				{Role: RoleAssistant, Content: "hi"},
			},
		},
		{
			name: "single system message",
			msgs: []LLMMessage{
				{Role: RoleSystem, Content: "be helpful"},
				{Role: RoleUser, Content: "hello"},
			},
			want: []LLMMessage{
				{Role: RoleSystem, Content: "be helpful"},
				{Role: RoleUser, Content: "hello"},
			},
		},
		{
			name: "multiple system messages - keep first",
			msgs: []LLMMessage{
				{Role: RoleSystem, Content: "first"},
				{Role: RoleUser, Content: "hello"},
				{Role: RoleSystem, Content: "second"},
				{Role: RoleAssistant, Content: "hi"},
				{Role: RoleSystem, Content: "third"},
			},
			want: []LLMMessage{
				{Role: RoleSystem, Content: "first"},
				{Role: RoleUser, Content: "hello"},
				{Role: RoleAssistant, Content: "hi"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeduplicateSystemMessages(tt.msgs)
			assert.Equal(t, tt.want, got)
		})
	}
}
