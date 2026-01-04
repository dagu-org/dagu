package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/dagu-org/dagu/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRequestBody_ThinkingTokens(t *testing.T) {
	t.Parallel()

	provider := &Provider{
		config: llm.Config{APIKey: "test-key"},
	}

	tests := []struct {
		name                string
		maxTokens           *int
		thinking            *llm.ThinkingRequest
		expectedMaxTokens   int
		expectedHasThinking bool
		expectedBudget      int
	}{
		{
			name:                "no thinking, no max tokens - default 4096",
			maxTokens:           nil,
			thinking:            nil,
			expectedMaxTokens:   4096,
			expectedHasThinking: false,
		},
		{
			name:                "no thinking, explicit max tokens",
			maxTokens:           intPtr(8192),
			thinking:            nil,
			expectedMaxTokens:   8192,
			expectedHasThinking: false,
		},
		{
			name:      "thinking enabled, no max tokens - auto adjust",
			maxTokens: nil,
			thinking: &llm.ThinkingRequest{
				Enabled: true,
				Effort:  llm.ThinkingEffortMedium, // 4096 budget
			},
			expectedMaxTokens:   4096 + 4096, // budget + buffer
			expectedHasThinking: true,
			expectedBudget:      4096,
		},
		{
			name:      "thinking enabled, max tokens less than budget - auto adjust",
			maxTokens: intPtr(2000),
			thinking: &llm.ThinkingRequest{
				Enabled: true,
				Effort:  llm.ThinkingEffortMedium, // 4096 budget
			},
			expectedMaxTokens:   4096 + 4096, // budget + buffer
			expectedHasThinking: true,
			expectedBudget:      4096,
		},
		{
			name:      "thinking enabled, max tokens equal to budget - auto adjust",
			maxTokens: intPtr(4096),
			thinking: &llm.ThinkingRequest{
				Enabled: true,
				Effort:  llm.ThinkingEffortMedium, // 4096 budget
			},
			expectedMaxTokens:   4096 + 4096, // budget + buffer
			expectedHasThinking: true,
			expectedBudget:      4096,
		},
		{
			name:      "thinking enabled, max tokens greater than budget - use provided",
			maxTokens: intPtr(16000),
			thinking: &llm.ThinkingRequest{
				Enabled: true,
				Effort:  llm.ThinkingEffortMedium, // 4096 budget
			},
			expectedMaxTokens:   16000,
			expectedHasThinking: true,
			expectedBudget:      4096,
		},
		{
			name:      "thinking with explicit budget tokens",
			maxTokens: nil,
			thinking: &llm.ThinkingRequest{
				Enabled:      true,
				BudgetTokens: intPtr(10000),
			},
			expectedMaxTokens:   10000 + 4096, // budget + buffer
			expectedHasThinking: true,
			expectedBudget:      10000,
		},
		{
			name:      "thinking high effort",
			maxTokens: nil,
			thinking: &llm.ThinkingRequest{
				Enabled: true,
				Effort:  llm.ThinkingEffortHigh, // 16384 budget
			},
			expectedMaxTokens:   16384 + 4096, // budget + buffer
			expectedHasThinking: true,
			expectedBudget:      16384,
		},
		{
			name:      "thinking xhigh effort",
			maxTokens: nil,
			thinking: &llm.ThinkingRequest{
				Enabled: true,
				Effort:  llm.ThinkingEffortXHigh, // 32768 budget
			},
			expectedMaxTokens:   32768 + 4096, // budget + buffer
			expectedHasThinking: true,
			expectedBudget:      32768,
		},
		{
			name:      "thinking low effort - default max tokens sufficient",
			maxTokens: nil,
			thinking: &llm.ThinkingRequest{
				Enabled: true,
				Effort:  llm.ThinkingEffortLow, // 1024 budget
			},
			// Default 4096 is already > 1024, so no adjustment needed
			expectedMaxTokens:   4096,
			expectedHasThinking: true,
			expectedBudget:      1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := &llm.ChatRequest{
				Model: "claude-sonnet-4-20250514",
				Messages: []llm.Message{
					{Role: llm.RoleUser, Content: "Hello"},
				},
				MaxTokens: tt.maxTokens,
				Thinking:  tt.thinking,
			}

			body, err := provider.buildRequestBody(req, false)
			require.NoError(t, err)

			var parsed messagesRequest
			err = json.Unmarshal(body, &parsed)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedMaxTokens, parsed.MaxTokens,
				"max_tokens should be %d, got %d", tt.expectedMaxTokens, parsed.MaxTokens)

			if tt.expectedHasThinking {
				require.NotNil(t, parsed.Thinking, "thinking should be present")
				assert.Equal(t, "enabled", parsed.Thinking.Type)
				assert.Equal(t, tt.expectedBudget, parsed.Thinking.BudgetToken,
					"budget_tokens should be %d, got %d", tt.expectedBudget, parsed.Thinking.BudgetToken)
				assert.Greater(t, parsed.MaxTokens, parsed.Thinking.BudgetToken,
					"max_tokens (%d) must be greater than budget_tokens (%d)",
					parsed.MaxTokens, parsed.Thinking.BudgetToken)
			} else {
				assert.Nil(t, parsed.Thinking, "thinking should not be present")
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}
