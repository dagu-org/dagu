package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAskUserRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          AskUserToolInput
		response       UserPromptResponse
		expectError    bool
		expectedOutput string
	}{
		{
			name: "single selection",
			input: AskUserToolInput{
				Question: "Choose one",
				Options:  []string{"Option A", "Option B"},
			},
			response: UserPromptResponse{
				PromptID:          "test-prompt",
				SelectedOptionIDs: []string{"opt_0"},
			},
			expectedOutput: "User selected: Option A",
		},
		{
			name: "multi selection",
			input: AskUserToolInput{
				Question:    "Choose multiple",
				Options:     []string{"Option A", "Option B", "Option C"},
				MultiSelect: true,
			},
			response: UserPromptResponse{
				PromptID:          "test-prompt",
				SelectedOptionIDs: []string{"opt_0", "opt_2"},
			},
			expectedOutput: "User selected: Option A, Option C",
		},
		{
			name: "free text response",
			input: AskUserToolInput{
				Question:      "What is your name?",
				AllowFreeText: true,
			},
			response: UserPromptResponse{
				PromptID:         "test-prompt",
				FreeTextResponse: "John Doe",
			},
			expectedOutput: "User responded: John Doe",
		},
		{
			name: "cancelled",
			input: AskUserToolInput{
				Question: "Choose one",
				Options:  []string{"Option A", "Option B"},
			},
			response: UserPromptResponse{
				PromptID:  "test-prompt",
				Cancelled: true,
			},
			expectedOutput: "User skipped this question",
		},
		{
			name: "empty question",
			input: AskUserToolInput{
				Question: "",
				Options:  []string{"A", "B"},
			},
			expectError: true,
		},
		{
			name: "invalid options count - one",
			input: AskUserToolInput{
				Question: "Choose",
				Options:  []string{"Only one"},
			},
			expectError: true,
		},
		{
			name: "invalid options count - five",
			input: AskUserToolInput{
				Question: "Choose",
				Options:  []string{"A", "B", "C", "D", "E"},
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var emittedPrompt UserPrompt
			emitFunc := func(prompt UserPrompt) {
				emittedPrompt = prompt
			}

			waitFunc := func(_ context.Context, _ string) (UserPromptResponse, error) {
				return tc.response, nil
			}

			ctx := ToolContext{
				Context:          context.Background(),
				EmitUserPrompt:   emitFunc,
				WaitUserResponse: waitFunc,
			}

			inputJSON, err := json.Marshal(tc.input)
			require.NoError(t, err)

			result := askUserRun(ctx, inputJSON)

			if tc.expectError {
				assert.True(t, result.IsError)
			} else {
				assert.False(t, result.IsError)
				assert.Equal(t, tc.expectedOutput, result.Content)
				assert.Equal(t, tc.input.Question, emittedPrompt.Question)
			}
		})
	}
}

func TestAskUserRunMissingCallbacks(t *testing.T) {
	t.Parallel()

	input := AskUserToolInput{
		Question: "Test question",
		Options:  []string{"A", "B"},
	}

	inputJSON, err := json.Marshal(input)
	require.NoError(t, err)

	ctx := ToolContext{
		Context: context.Background(),
		// No EmitUserPrompt or WaitUserResponse
	}

	result := askUserRun(ctx, inputJSON)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "not available")
}

func TestFormatUserResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     AskUserToolInput
		response UserPromptResponse
		expected string
	}{
		{
			name: "selection and free text",
			args: AskUserToolInput{
				Options: []string{"Option A", "Option B"},
			},
			response: UserPromptResponse{
				SelectedOptionIDs: []string{"opt_0"},
				FreeTextResponse:  "Additional info",
			},
			expected: "User selected: Option A\nUser responded: Additional info",
		},
		{
			name: "no response",
			args: AskUserToolInput{
				Options: []string{"Option A", "Option B"},
			},
			response: UserPromptResponse{},
			expected: "User provided no response",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := formatUserResponse(tc.args, tc.response)
			assert.Equal(t, tc.expected, result)
		})
	}
}
