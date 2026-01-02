package execution

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLLMMessages(t *testing.T) {
	msgs := NewLLMMessages()
	require.NotNil(t, msgs)
	assert.NotNil(t, msgs.Steps)
	assert.Empty(t, msgs.Steps)
}

func TestLLMMessages_GetSetStepMessages(t *testing.T) {
	msgs := NewLLMMessages()

	// Get from empty returns nil
	assert.Nil(t, msgs.GetStepMessages("step1"))

	// Set and get
	messages := []LLMMessage{
		{Role: RoleUser, Content: "hello"},
		{Role: RoleAssistant, Content: "hi"},
	}
	msgs.SetStepMessages("step1", messages)

	got := msgs.GetStepMessages("step1")
	require.Len(t, got, 2)
	assert.Equal(t, RoleUser, got[0].Role)
	assert.Equal(t, "hello", got[0].Content)

	// Get non-existent step
	assert.Nil(t, msgs.GetStepMessages("step2"))
}

func TestLLMMessages_GetStepMessages_NilReceiver(t *testing.T) {
	var msgs *LLMMessages
	assert.Nil(t, msgs.GetStepMessages("step1"))
}

func TestLLMMessages_SetStepMessages_NilSteps(t *testing.T) {
	msgs := &LLMMessages{}
	msgs.SetStepMessages("step1", []LLMMessage{{Role: RoleUser, Content: "test"}})
	assert.NotNil(t, msgs.Steps)
	assert.Len(t, msgs.Steps["step1"], 1)
}

func TestLLMMessages_MergeFromDependencies(t *testing.T) {
	tests := []struct {
		name     string
		msgs     *LLMMessages
		depends  []string
		wantLen  int
		wantMsgs []LLMMessage
	}{
		{
			name:    "nil receiver",
			msgs:    nil,
			depends: []string{"step1"},
			wantLen: 0,
		},
		{
			name:    "nil steps",
			msgs:    &LLMMessages{},
			depends: []string{"step1"},
			wantLen: 0,
		},
		{
			name: "empty depends",
			msgs: &LLMMessages{
				Steps: map[string][]LLMMessage{
					"step1": {{Role: RoleUser, Content: "hello"}},
				},
			},
			depends: []string{},
			wantLen: 0,
		},
		{
			name: "single dependency",
			msgs: &LLMMessages{
				Steps: map[string][]LLMMessage{
					"step1": {
						{Role: RoleSystem, Content: "you are helpful"},
						{Role: RoleUser, Content: "hello"},
						{Role: RoleAssistant, Content: "hi"},
					},
				},
			},
			depends: []string{"step1"},
			wantLen: 3,
		},
		{
			name: "multiple dependencies merged",
			msgs: &LLMMessages{
				Steps: map[string][]LLMMessage{
					"step1": {
						{Role: RoleUser, Content: "q1"},
						{Role: RoleAssistant, Content: "a1"},
					},
					"step2": {
						{Role: RoleUser, Content: "q2"},
						{Role: RoleAssistant, Content: "a2"},
					},
				},
			},
			depends: []string{"step1", "step2"},
			wantLen: 4,
		},
		{
			name: "system messages deduplicated",
			msgs: &LLMMessages{
				Steps: map[string][]LLMMessage{
					"step1": {
						{Role: RoleSystem, Content: "sys1"},
						{Role: RoleUser, Content: "q1"},
					},
					"step2": {
						{Role: RoleSystem, Content: "sys2"},
						{Role: RoleUser, Content: "q2"},
					},
				},
			},
			depends: []string{"step1", "step2"},
			wantLen: 3, // Only first system message kept
			wantMsgs: []LLMMessage{
				{Role: RoleSystem, Content: "sys1"},
				{Role: RoleUser, Content: "q1"},
				{Role: RoleUser, Content: "q2"},
			},
		},
		{
			name: "missing dependency ignored",
			msgs: &LLMMessages{
				Steps: map[string][]LLMMessage{
					"step1": {{Role: RoleUser, Content: "hello"}},
				},
			},
			depends: []string{"step1", "step_missing"},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.msgs.MergeFromDependencies(tt.depends)
			assert.Len(t, got, tt.wantLen)
			if tt.wantMsgs != nil {
				assert.Equal(t, tt.wantMsgs, got)
			}
		})
	}
}

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
